package app

import (
	"reflect"
	"testing"

	"oneimg/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateLegacyUserPermissions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		id, role int
		name     string
		raw      string
	}{
		{1, models.RoleAdmin, "legacy-admin", `{"buckets":[2]}`},
		{2, models.RoleAdmin, "restricted-admin", `{"codes":[],"buckets":[3]}`},
		{3, models.RoleUser, "legacy-user", `{"buckets":[4]}`},
	}
	for _, row := range rows {
		if err := db.Exec("INSERT INTO users (id, role, username, password, permission) VALUES (?, ?, ?, ?, ?)", row.id, row.role, row.name, "hash", row.raw).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := MigrateLegacyUserPermissions(db); err != nil {
		t.Fatal(err)
	}

	var users []models.User
	if err := db.Order("id").Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(users[0].Permission.Codes, models.AllPermissionCodes()) {
		t.Fatalf("legacy admin codes = %v", users[0].Permission.Codes)
	}
	if !reflect.DeepEqual(users[0].Permission.Buckets, []int{2}) {
		t.Fatalf("legacy admin buckets = %v", users[0].Permission.Buckets)
	}
	if len(users[1].Permission.Codes) != 0 || !reflect.DeepEqual(users[1].Permission.Buckets, []int{3}) {
		t.Fatalf("explicit empty permission changed: %+v", users[1].Permission)
	}
	if len(users[2].Permission.Codes) != 0 || !reflect.DeepEqual(users[2].Permission.Buckets, []int{4}) {
		t.Fatalf("normal user permission changed: %+v", users[2].Permission)
	}
}
