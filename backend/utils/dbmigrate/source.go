package dbmigrate

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/passkeys"
	"oneimg/backend/utils/secureconfig"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type table struct {
	name       string
	fields     []*schema.Field
	keys       []string
	indexes    []string
	uniqueKeys [][]string
}

func tables(db *gorm.DB) ([]table, error) {
	result := make([]table, 0, 8)
	for _, model := range database.Models() {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(model); err != nil {
			return nil, fail(1, "cannot inspect application models")
		}
		item := table{name: statement.Table}
		for _, index := range statement.Schema.ParseIndexes() {
			item.indexes = append(item.indexes, index.Name)
			if index.Class == "UNIQUE" {
				var columns []string
				for _, column := range index.Fields {
					collation := strings.ToUpper(column.Collate)
					if collation == "" {
						collation = "BINARY"
					}
					columns = append(columns, column.DBName+" COLLATE "+collation)
				}
				item.uniqueKeys = append(item.uniqueKeys, columns)
			}
		}
		for _, field := range statement.Schema.Fields {
			if field.DBName == "" {
				continue
			}
			item.fields = append(item.fields, field)
			if field.PrimaryKey {
				item.keys = append(item.keys, field.DBName)
			}
			if field.Unique {
				item.uniqueKeys = append(item.uniqueKeys, []string{field.DBName + " COLLATE BINARY"})
			}
		}
		if item.name == "passkey_credentials" {
			item.uniqueKeys = append(item.uniqueKeys, []string{"user_id COLLATE BINARY", "name COLLATE NOCASE"})
		}
		sort.Slice(item.fields, func(i, j int) bool { return item.fields[i].DBName < item.fields[j].DBName })
		result = append(result, item)
	}
	// Fixed dependency order, independent of model declaration order.
	order := map[string]int{"users": 0, "buckets": 1, "settings": 2, "tags": 3, "images": 4, "image_to_tags": 5, "passkey_credentials": 6, "personal_access_tokens": 7}
	sort.Slice(result, func(i, j int) bool { return order[result[i].name] < order[result[j].name] })
	return result, nil
}

func openSource(path string) (*database.Database, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fail(2, "invalid source path")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fail(3, "source must be a regular SQLite backup")
	}
	if wal, err := os.Stat(absolute + "-wal"); err == nil && wal.Size() > 0 {
		return nil, fail(3, "source has a nonempty WAL; create a consistent SQLite backup first")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, fail(3, "cannot inspect source WAL")
	}
	location := url.URL{Scheme: "file", Path: absolute}
	query := location.Query()
	query.Set("mode", "ro")
	query.Set("_query_only", "1")
	location.RawQuery = query.Encode()
	db, err := database.NewDB(sqlite.Open(location.String()))
	if err != nil {
		return nil, fail(3, "cannot open read-only SQLite source")
	}
	connection, _ := db.DB.DB()
	connection.SetMaxOpenConns(1)
	return db, nil
}

func checkSource(tx *gorm.DB, specs []table, options Options, cfg *config.Config) (map[string]int64, error) {
	var integrity string
	rows, err := tx.Raw("PRAGMA integrity_check").Rows()
	if err != nil {
		return nil, fail(3, "source integrity check failed")
	}
	for rows.Next() {
		if err := rows.Scan(&integrity); err != nil || integrity != "ok" {
			_ = rows.Close()
			return nil, fail(3, "source integrity check failed")
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, fail(3, "source integrity check failed")
	}
	known := map[string]bool{"sqlite_sequence": true, "oneimg_schema_migrations": true, "image_storages": true}
	for _, spec := range specs {
		known[spec.name] = true
	}
	indexes := map[string]bool{"idx_passkey_user_name": true}
	for _, spec := range specs {
		for _, name := range spec.indexes {
			indexes[name] = true
		}
	}
	var objects []struct {
		Name, Type string
		Table      string `gorm:"column:tbl_name"`
	}
	if err := tx.Raw("SELECT name,type,tbl_name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'").Scan(&objects).Error; err != nil {
		return nil, fail(3, "cannot inspect source schema")
	}
	for _, object := range objects {
		if object.Type == "index" {
			if !indexes[object.Name] && object.Table != "image_storages" {
				return nil, fail(3, "source contains unrecognized indexes")
			}
			continue
		}
		if object.Type != "table" || !known[object.Name] {
			return nil, fail(3, "source contains unrecognized schema objects")
		}
		if object.Name == "image_storages" {
			fmt.Fprintln(options.Output, "obsolete table image_storages: retained in source, not imported")
		}
	}
	for _, spec := range specs {
		var columns []struct {
			Name, Type string
			Notnull    int
			Pk         int
		}
		if err := tx.Raw(`PRAGMA table_info("` + spec.name + `")`).Scan(&columns).Error; err != nil {
			return nil, fail(3, "cannot inspect source columns")
		}
		expected := map[string]*schema.Field{}
		for _, field := range spec.fields {
			expected[field.DBName] = field
		}
		for _, column := range columns {
			field, ok := expected[column.Name]
			if !ok {
				if spec.name == "settings" && column.Name == "multi_storage_sync" {
					fmt.Fprintln(options.Output, "obsolete column settings.multi_storage_sync: not imported")
					continue
				}
				return nil, fail(3, "source contains unrecognized columns")
			}
			if !compatibleType(field, column.Type) {
				return nil, fail(3, fmt.Sprintf("incompatible source column %s.%s", spec.name, column.Name))
			}
			position := 0
			for index, key := range spec.keys {
				if key == column.Name {
					position = index + 1
				}
			}
			if column.Pk != position || (field.NotNull && !field.PrimaryKey && column.Notnull == 0) {
				return nil, fail(3, "source column constraints are incompatible: "+spec.name+"."+column.Name)
			}
			delete(expected, column.Name)
		}
		if len(expected) > 0 {
			return nil, fail(3, "source is missing current columns; upgrade SQLite with this version first")
		}
		if err := checkSourceConstraints(tx, spec); err != nil {
			return nil, err
		}
	}
	var metadataColumns []struct{ Name string }
	if err := tx.Raw(`PRAGMA table_info("oneimg_schema_migrations")`).Scan(&metadataColumns).Error; err != nil || len(metadataColumns) != 2 {
		return nil, fail(3, "source migration metadata is incompatible")
	}
	for _, column := range metadataColumns {
		if column.Name != "id" && column.Name != "applied_at" {
			return nil, fail(3, "source contains extended migration metadata")
		}
	}
	var markers int64
	if err := tx.Raw("SELECT COUNT(*) FROM oneimg_schema_migrations WHERE id IN (?,?)", database.SchemaVersion, database.SearchVersion).Scan(&markers).Error; err != nil || markers != 2 {
		return nil, fail(3, "source has not completed the current SQLite upgrade")
	}
	var unknownMarkers int64
	if err := tx.Raw("SELECT COUNT(*) FROM oneimg_schema_migrations WHERE id NOT IN (?,?)", database.SchemaVersion, database.SearchVersion).Scan(&unknownMarkers).Error; err != nil || unknownMarkers != 0 {
		return nil, fail(3, "source contains unsupported migration versions")
	}
	if strings.TrimSpace(cfg.ConfigSecret) == "" {
		return nil, fail(2, "original CONFIG_SECRET is required")
	}
	previous := config.App
	config.App = cfg
	defer func() { config.App = previous }()
	var buckets []models.Buckets
	if err := tx.Find(&buckets).Error; err != nil {
		return nil, fail(3, "source bucket configuration is invalid")
	}
	local := false
	for _, bucket := range buckets {
		if bucket.Type == "default" && bucket.Id == 1 {
			local = true
		}
		if _, err := secureconfig.DecryptBucketConfigValues(bucket.Config); err != nil {
			return nil, fail(3, fmt.Sprintf("cannot decrypt buckets row %d", bucket.Id))
		}
	}
	if !local {
		return nil, fail(3, "source requires the current built-in local bucket")
	}
	var admin models.User
	if err := tx.First(&admin, models.SuperAdminID).Error; err != nil || admin.Role != models.RoleAdmin || admin.AuthVersion < 1 {
		return nil, fail(3, "source super administrator is invalid")
	}
	var settings []models.Settings
	if err := tx.Find(&settings).Error; err != nil || len(settings) != 1 || settings[0].ID != 1 {
		return nil, fail(3, "source must contain current singleton settings")
	}
	defaultFound := false
	for _, bucket := range buckets {
		if bucket.Id == settings[0].DefaultStorage && !bucket.Disabled {
			defaultFound = true
		}
	}
	if !defaultFound {
		return nil, fail(3, "source default storage is unavailable")
	}
	var credentials []models.PasskeyCredential
	if err := tx.FindInBatches(&credentials, 250, func(_ *gorm.DB, _ int) error {
		for _, credential := range credentials {
			if _, err := passkeys.DecodeCredential(credential); err != nil {
				return fail(3, fmt.Sprintf("cannot decrypt passkey_credentials row %d", credential.ID))
			}
		}
		return nil
	}).Error; err != nil {
		return nil, fail(3, "source Passkey data or encryption key is invalid")
	}
	high := map[string]int64{}
	var sequence []struct {
		Name string
		Seq  int64
	}
	if err := tx.Raw("SELECT name,seq FROM sqlite_sequence").Scan(&sequence).Error; err != nil {
		return nil, fail(3, "cannot inspect source ID high-water marks")
	}
	for _, item := range sequence {
		if !known[item.Name] || item.Seq < 0 {
			return nil, fail(3, "invalid source ID high-water mark")
		}
		high[item.Name] = item.Seq
	}
	return high, nil
}

func checkSourceConstraints(tx *gorm.DB, spec table) error {
	var foreignKeys []struct{ ID int }
	if err := tx.Raw("PRAGMA foreign_key_list(" + tx.Statement.Quote(spec.name) + ")").Scan(&foreignKeys).Error; err != nil || len(foreignKeys) != 0 {
		return fail(3, "source has incompatible foreign key constraints: "+spec.name)
	}
	var indexes []struct {
		Name, Origin string
		Unique       int
		Partial      int
	}
	if err := tx.Raw("PRAGMA index_list(" + tx.Statement.Quote(spec.name) + ")").Scan(&indexes).Error; err != nil {
		return fail(3, "cannot inspect source constraints: "+spec.name)
	}
	required := map[string]bool{}
	for _, columns := range spec.uniqueKeys {
		required[strings.Join(columns, ",")] = false
	}
	for _, index := range indexes {
		if index.Unique == 0 || index.Origin == "pk" {
			continue
		}
		var columns []struct {
			Name, Coll string
			Key        int
		}
		if err := tx.Raw("PRAGMA index_xinfo(" + tx.Statement.Quote(index.Name) + ")").Scan(&columns).Error; err != nil {
			return fail(3, "cannot inspect source unique constraint: "+spec.name)
		}
		var names []string
		for _, column := range columns {
			if column.Key == 1 {
				names = append(names, column.Name+" COLLATE "+strings.ToUpper(column.Coll))
			}
		}
		signature := strings.Join(names, ",")
		if _, known := required[signature]; !known || index.Partial != 0 {
			return fail(3, "source contains incompatible unique constraints: "+spec.name)
		}
		required[signature] = true
	}
	for _, found := range required {
		if !found {
			return fail(3, "source is missing current unique constraints: "+spec.name)
		}
	}
	return nil
}

func compatibleType(field *schema.Field, declared string) bool {
	value := strings.ToLower(declared)
	if field.DBName == "permission" {
		return value == "jsonb"
	}
	switch field.GORMDataType {
	case schema.Int, schema.Uint:
		return strings.Contains(value, "int")
	case schema.Bool:
		return value == "numeric" || value == "boolean" || value == "integer"
	case schema.Time:
		return value == "datetime" || strings.HasPrefix(value, "timestamp")
	default:
		return value == "text" || strings.HasPrefix(value, "varchar")
	}
}

func sourceTransaction(db *database.Database) *gorm.DB {
	return db.DB.Begin(&sql.TxOptions{ReadOnly: true})
}
