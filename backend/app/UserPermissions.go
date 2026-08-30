package app

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"oneimg/backend/models"

	"gorm.io/gorm"
)

// MigrateLegacyUserPermissions adds the codes key once while preserving the
// distinction between legacy permissions and an explicitly empty code list.
func MigrateLegacyUserPermissions(db *gorm.DB) error {
	rows, err := db.Raw("SELECT id, role, permission FROM users").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	type legacyPermission struct {
		Buckets []int `json:"buckets"`
	}
	type permissionUpdate struct {
		ID         int
		Permission models.Permission
	}
	updates := make([]permissionUpdate, 0)
	for rows.Next() {
		var id, role int
		var raw sql.NullString
		if err := rows.Scan(&id, &role, &raw); err != nil {
			return err
		}

		permissionJSON := raw.String
		if permissionJSON == "" || permissionJSON == "null" || permissionJSON == "[]" {
			permissionJSON = "{}"
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(permissionJSON), &fields); err != nil {
			return fmt.Errorf("解析用户 %d 权限失败: %w", id, err)
		}
		if _, exists := fields["codes"]; exists {
			continue
		}

		var legacy legacyPermission
		if err := json.Unmarshal([]byte(permissionJSON), &legacy); err != nil {
			return fmt.Errorf("解析用户 %d 存储权限失败: %w", id, err)
		}
		if legacy.Buckets == nil {
			legacy.Buckets = []int{}
		}
		codes := []string{}
		if role == models.RoleAdmin {
			codes = models.AllPermissionCodes()
		}
		updates = append(updates, permissionUpdate{ID: id, Permission: models.Permission{Codes: codes, Buckets: legacy.Buckets}})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, update := range updates {
		if err := db.Model(&models.User{}).Where("id = ?", update.ID).Update("permission", update.Permission).Error; err != nil {
			return fmt.Errorf("迁移用户 %d 权限失败: %w", update.ID, err)
		}
	}
	return nil
}

// InvalidateLegacyAPIAccess performs the destructive v1 cutover. Legacy
// global tokens and their permission code are intentionally not recoverable.
func InvalidateLegacyAPIAccess(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Settings{}).Where("1 = 1").Updates(map[string]any{
			"start_api": false, "api_token": "", "api_token_hash": "",
		}).Error; err != nil {
			return err
		}
		var users []models.User
		if err := tx.Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			filtered := make([]string, 0, len(user.Permission.Codes))
			changed := false
			for _, code := range user.Permission.Codes {
				if code == "setting:api" {
					changed = true
					continue
				}
				filtered = append(filtered, code)
			}
			if changed {
				user.Permission.Codes = filtered
				if err := tx.Model(&user).Update("permission", user.Permission).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
