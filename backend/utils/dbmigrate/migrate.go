package dbmigrate

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/database"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const importVersion = "sqlite_import_v1"

func Run(cfg *config.Config, options Options) error {
	if cfg == nil || cfg.DatabaseType() != "postgres" || options.Source == "" || (options.DryRun && options.VerifyOnly) {
		return fail(2, "invalid migration configuration")
	}
	if _, err := database.PostgreSQLConfig(cfg.DatabaseURL); err != nil {
		return fail(2, err.Error())
	}
	target, err := database.OpenPostgres(cfg, "public")
	if err != nil {
		return fail(1, "cannot connect to migration target")
	}
	defer target.Close()
	return run(cfg, options, target.DB)
}

func run(cfg *config.Config, options Options, target *gorm.DB) error {
	if options.Output == nil {
		options.Output = io.Discard
	}
	source, err := openSource(options.Source)
	if err != nil {
		return err
	}
	defer source.Close()
	snapshot := sourceTransaction(source)
	if snapshot.Error != nil {
		return fail(1, "cannot start source snapshot")
	}
	defer snapshot.Rollback()
	specs, err := tables(snapshot)
	if err != nil {
		return err
	}
	high, err := checkSource(snapshot, specs, options, cfg)
	if err != nil {
		return err
	}
	var precision int64
	// Complete validation precedes target DDL, including every source value.
	for _, spec := range specs {
		rows, err := selectRows(snapshot, spec, true)
		if err != nil {
			return fail(3, "cannot scan source table: "+spec.name)
		}
		var count int64
		for rows.Next() {
			_, normalized, err := scanRow(rows, spec, true)
			if err != nil {
				_ = rows.Close()
				return err
			}
			precision += normalized
			count++
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return fail(3, "source scan failed: "+spec.name)
		}
		fmt.Fprintf(options.Output, "source %s rows=%d validated\n", spec.name, count)
		if high[spec.name] == math.MaxInt64 {
			return fail(3, "source ID sequence is exhausted: "+spec.name)
		}
	}
	fmt.Fprintf(options.Output, "timestamps normalized to UTC microseconds fields=%d\n", precision)
	if err := reportOrphans(snapshot, options.Output); err != nil {
		return err
	}
	if options.VerifyOnly {
		tx := target.Begin(&sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
		if tx.Error != nil {
			return fail(1, "cannot start read-only verification")
		}
		defer tx.Rollback()
		var marker int64
		if err := tx.Model(&database.SchemaMigration{}).Where("id = ?", importVersion).Count(&marker).Error; err != nil || marker != 1 {
			return fail(3, "target has no completed SQLite import")
		}
		if err := verify(snapshot, tx, specs, high, options); err != nil {
			return err
		}
		return nil
	}
	if options.DryRun {
		tx := target.Begin(&sql.TxOptions{ReadOnly: true})
		if tx.Error != nil {
			return fail(1, "cannot start read-only preflight")
		}
		defer tx.Rollback()
		return emptyTarget(tx)
	}
	tx := target.Begin()
	if tx.Error != nil {
		return fail(1, "cannot start target transaction")
	}
	defer tx.Rollback()
	if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", database.MigrationLock).Error; err != nil {
		return fail(1, "cannot acquire import lock")
	}
	if err := emptyTarget(tx); err != nil {
		return err
	}
	if err := database.MigrateSchema(tx); err != nil {
		return fail(1, "target schema creation failed")
	}
	for _, spec := range specs {
		rows, err := selectRows(snapshot, spec, true)
		if err != nil {
			return fail(1, "cannot copy source table: "+spec.name)
		}
		batch := make([]map[string]any, 0, 250)
		write := func() error {
			if len(batch) == 0 {
				return nil
			}
			if err := tx.Table(spec.name).Create(&batch).Error; err != nil {
				return fail(1, "target copy failed: "+spec.name+" ("+database.ErrorCode(err)+")")
			}
			batch = batch[:0]
			return nil
		}
		for rows.Next() {
			row, _, err := scanRow(rows, spec, true)
			if err != nil {
				_ = rows.Close()
				return err
			}
			batch = append(batch, row)
			if len(batch) == 250 {
				if err := write(); err != nil {
					_ = rows.Close()
					return err
				}
			}
		}
		readErr := rows.Err()
		_ = rows.Close()
		if readErr != nil {
			return fail(1, "source copy read failed")
		}
		if err := write(); err != nil {
			return err
		}
		if options.hook != nil {
			if err := options.hook("copied:"+spec.name, tx); err != nil {
				return fail(1, "injected copy interruption")
			}
		}
	}
	if options.hook != nil {
		if err := options.hook("before-verify", tx); err != nil {
			return fail(1, "verification interrupted")
		}
	}
	if err := database.SyncSequences(tx, high); err != nil {
		return fail(1, "sequence synchronization failed")
	}
	if err := verify(snapshot, tx, specs, high, options); err != nil {
		return err
	}
	if err := database.MarkMigration(tx, importVersion); err != nil {
		return fail(1, "cannot record import completion")
	}
	err = tx.Commit().Error
	if err == nil && options.commit != nil {
		err = options.commit()
	}
	if err != nil {
		var known *pgconn.PgError
		if errors.As(err, &known) && (known.Severity == "ERROR" || known.SeverityUnlocalized == "ERROR") && !strings.HasPrefix(known.Code, "08") {
			return fail(1, "target commit rejected")
		}
		return fail(4, "target commit result is uncertain")
	}
	fmt.Fprintln(options.Output, "import committed; retain source and run --verify-only before switching")
	return nil
}

func emptyTarget(tx *gorm.DB) error {
	empty, err := database.SchemaEmpty(tx)
	if err != nil {
		return fail(1, "cannot inspect target schema")
	}
	if !empty {
		return fail(3, "target schema must be empty")
	}
	var allowed bool
	if err := tx.Raw("SELECT has_schema_privilege(current_user,current_schema(),'CREATE')").Scan(&allowed).Error; err != nil || !allowed {
		return fail(3, "target requires schema CREATE privilege")
	}
	return nil
}

func verify(source, target *gorm.DB, specs []table, high map[string]int64, options Options) error {
	for _, spec := range specs {
		count, err := compareTable(source, target, spec)
		if err != nil {
			return err
		}
		fmt.Fprintf(options.Output, "verified %s rows=%d\n", spec.name, count)
	}
	if err := database.CheckSequences(target, high); err != nil {
		return fail(1, "target sequence verification failed")
	}
	fmt.Fprintln(options.Output, "all records, references, authentication fields and sequences verified")
	return nil
}

func reportOrphans(tx *gorm.DB, output io.Writer) error {
	for _, item := range []struct{ name, query string }{
		{"images.user_id", "SELECT COUNT(*) FROM images i WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id=i.user_id)"},
		{"images.bucket_id", "SELECT COUNT(*) FROM images i WHERE NOT EXISTS (SELECT 1 FROM buckets b WHERE b.id=i.bucket_id)"},
		{"image_to_tags", "SELECT COUNT(*) FROM image_to_tags l WHERE NOT EXISTS (SELECT 1 FROM images i WHERE i.id=l.image_id) OR NOT EXISTS (SELECT 1 FROM tags t WHERE t.id=l.tag_id)"},
		{"personal_access_tokens.user_id", "SELECT COUNT(*) FROM personal_access_tokens p WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id=p.user_id)"},
		{"passkey_credentials.user_id", "SELECT COUNT(*) FROM passkey_credentials p WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id=p.user_id)"},
	} {
		var count int64
		if err := tx.Raw(item.query).Scan(&count).Error; err != nil {
			return fail(3, "cannot inspect source references: "+item.name)
		}
		if count > 0 {
			fmt.Fprintf(output, "historical dangling references %s rows=%d preserved\n", item.name, count)
		}
	}
	var count int64
	if err := tx.Raw(`SELECT COUNT(*) FROM users u, json_each(u.permission, '$.buckets') p
WHERE NOT EXISTS (SELECT 1 FROM buckets b WHERE b.id=p.value)`).Scan(&count).Error; err != nil {
		return fail(3, "cannot inspect source permission bucket references")
	}
	if count > 0 {
		fmt.Fprintf(output, "historical dangling references users.permission.buckets references=%d preserved\n", count)
	}
	return nil
}
