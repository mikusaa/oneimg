package dbmigrate

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"oneimg/backend/models"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func fieldError(spec table, row map[string]any, column string) error {
	id := row["id"]
	if id == nil {
		id = row["image_id"]
	}
	if _, ok := id.(int64); !ok {
		id = "invalid"
	}
	return fail(3, fmt.Sprintf("incompatible data: %s row %v column %s", spec.name, id, column))
}

func selectRows(db *gorm.DB, spec table, source bool) (*sql.Rows, error) {
	columns := make([]string, 0, len(spec.fields))
	for _, field := range spec.fields {
		name := db.Statement.Quote(field.DBName)
		if source && field.GORMDataType == schema.Time {
			name = "CAST(" + name + " AS TEXT)"
		}
		columns = append(columns, name)
	}
	order := make([]string, 0, len(spec.keys))
	for _, key := range spec.keys {
		order = append(order, db.Statement.Quote(key))
	}
	return db.Raw("SELECT " + strings.Join(columns, ",") + " FROM " + db.Statement.Quote(spec.name) + " ORDER BY " + strings.Join(order, ",")).Rows()
}

func scanRow(rows *sql.Rows, spec table, source bool) (map[string]any, int64, error) {
	values := make([]any, len(spec.fields))
	pointers := make([]any, len(values))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := rows.Scan(pointers...); err != nil {
		return nil, 0, fail(1, "cannot scan migration row")
	}
	row := make(map[string]any, len(values))
	for i, field := range spec.fields {
		value := values[i]
		if bytes, ok := value.([]byte); ok {
			value = string(bytes)
		}
		row[field.DBName] = value
	}
	var precision int64
	for _, field := range spec.fields {
		value := row[field.DBName]
		if value == nil {
			if field.NotNull || field.PrimaryKey {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
			continue
		}
		switch field.GORMDataType {
		case schema.Time:
			timestamp, err := parseTime(value)
			if err != nil {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
			normalized := timestamp.UTC().Truncate(time.Microsecond)
			if !timestamp.Equal(normalized) {
				precision++
			}
			row[field.DBName] = normalized
		case schema.Int, schema.Uint:
			integer, ok := value.(int64)
			if !ok {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
			if integer < 0 {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
			if field.PrimaryKey && integer == math.MaxInt64 {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
			row[field.DBName] = integer
		case schema.Bool:
			switch v := value.(type) {
			case bool:
			case int64:
				if v != 0 && v != 1 {
					return nil, 0, fieldError(spec, row, field.DBName)
				}
				row[field.DBName] = v == 1
			default:
				return nil, 0, fieldError(spec, row, field.DBName)
			}
		default:
			text, ok := value.(string)
			if !ok || !utf8.ValidString(text) || strings.ContainsRune(text, 0) || (field.Size > 0 && utf8.RuneCountInString(text) > field.Size) {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
		}
		if spec.name == "users" && field.DBName == "permission" || spec.name == "buckets" && field.DBName == "config" || spec.name == "personal_access_tokens" && field.DBName == "scopes" {
			text, ok := row[field.DBName].(string)
			if !ok || !json.Valid([]byte(text)) || !validJSONStrings([]byte(text)) {
				return nil, 0, fieldError(spec, row, field.DBName)
			}
			if spec.name == "users" {
				var permission models.Permission
				if err := permission.Scan(text); err != nil {
					return nil, 0, fieldError(spec, row, field.DBName)
				}
			}
			if spec.name == "personal_access_tokens" {
				var list []string
				if err := json.Unmarshal([]byte(text), &list); err != nil {
					return nil, 0, fieldError(spec, row, field.DBName)
				}
			}
		}
	}
	if source && (spec.name == "users" || spec.name == "images") {
		// Search fields are recomputed under the current rule, never trusted.
		for original, derived := range map[string]string{"username": "username_search", "file_name": "file_name_search", "original_filename": "original_filename_search", "url": "url_search", "content_hash": "content_hash_search"} {
			if value, exists := row[original]; exists {
				text, _ := value.(string)
				row[derived] = models.SearchKey(text)
			}
		}
	}
	return row, precision, nil
}

func parseTime(value any) (time.Time, error) {
	if timestamp, ok := value.(time.Time); ok {
		return timestamp, nil
	}
	text, ok := value.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid timestamp")
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999", "2006-01-02T15:04:05.999999999", "2006-01-02"} {
		if value, err := time.Parse(layout, text); err == nil && value.Year() >= 1 && value.Year() <= 9999 {
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp")
}

func validJSONStrings(data []byte) bool {
	// JSON escapes can encode NUL or unpaired UTF-16 surrogates even when the
	// enclosing SQLite string is valid UTF-8. PostgreSQL JSONB rejects these.
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) || data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return false
		}
		n, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil || n == 0 {
			return false
		}
		i += 4
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(data) || string(data[i+1:i+3]) != "\\u" {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		} else if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
	}
	return true
}

func canonicalJSON(value any) any {
	switch v := value.(type) {
	case json.Number:
		number := new(big.Rat)
		if _, ok := number.SetString(string(v)); ok {
			return []any{"number", number.RatString()}
		}
	case map[string]any:
		for k, item := range v {
			v[k] = canonicalJSON(item)
		}
		return []any{"object", v}
	case []any:
		for i, item := range v {
			v[i] = canonicalJSON(item)
		}
		return []any{"array", v}
	case string:
		return []any{"string", v}
	case bool:
		return []any{"bool", v}
	case nil:
		return []any{"null"}
	}
	return value
}

func digest(spec table, row map[string]any) ([32]byte, error) {
	values := make([]any, 0, len(spec.fields))
	for _, field := range spec.fields {
		value := row[field.DBName]
		if spec.name == "users" && field.DBName == "permission" && value != nil {
			decoder := json.NewDecoder(strings.NewReader(value.(string)))
			decoder.UseNumber()
			var object any
			if err := decoder.Decode(&object); err != nil {
				return [32]byte{}, fail(1, "cannot normalize JSONB")
			}
			value = struct{ JSONB any }{canonicalJSON(object)}
		}
		values = append(values, value)
	}
	data, err := json.Marshal(values)
	if err != nil {
		return [32]byte{}, fail(1, "cannot normalize row")
	}
	return sha256.Sum256(data), nil
}

func compareTable(source, target *gorm.DB, spec table) (int64, error) {
	left, err := selectRows(source, spec, true)
	if err != nil {
		return 0, fail(1, "cannot read source for verification")
	}
	defer left.Close()
	right, err := selectRows(target, spec, false)
	if err != nil {
		return 0, fail(1, "cannot read target for verification")
	}
	defer right.Close()
	var count int64
	for left.Next() {
		if !right.Next() {
			return count, fail(1, "target row count differs: "+spec.name)
		}
		a, _, err := scanRow(left, spec, true)
		if err != nil {
			return count, err
		}
		b, _, err := scanRow(right, spec, false)
		if err != nil {
			return count, err
		}
		ad, err := digest(spec, a)
		if err != nil {
			return count, err
		}
		bd, err := digest(spec, b)
		if err != nil {
			return count, err
		}
		if ad != bd {
			for _, field := range spec.fields {
				one := table{name: spec.name, fields: []*schema.Field{field}}
				da, _ := digest(one, a)
				db, _ := digest(one, b)
				if da != db {
					return count, fail(1, fmt.Sprintf("verification differs: %s row %v column %s", spec.name, a[spec.keys[0]], field.DBName))
				}
			}
			return count, fail(1, "verification differs: "+spec.name)
		}
		count++
	}
	if left.Err() != nil || right.Err() != nil {
		return count, fail(1, "verification read failed: "+spec.name)
	}
	if right.Next() {
		return count, fail(1, "target row count differs: "+spec.name)
	}
	if right.Err() != nil {
		return count, fail(1, "verification read failed: "+spec.name)
	}
	return count, nil
}
