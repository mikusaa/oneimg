package services

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"oneimg/backend/models"
	"oneimg/backend/utils/localsecret"
)

func TestPasswordRotationAndTokenCreationRace(t *testing.T) {
	db := newServiceTestDB(t)
	user := createServiceTestUser(t, db, "owner", "old-password", models.RoleAdmin)
	other := createServiceTestUser(t, db, "other", "other-password", models.RoleUser)
	tokens := NewTokenService(db, "token-secret")
	input := CreateTokenInput{AuthVersion: user.AuthVersion, Name: "old", Scopes: []string{"images:read"}}
	token, err := tokens.Create(user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	passkey := models.PasskeyCredential{UserID: user.ID, Name: "retained", CredentialID: "key", CredentialData: "encrypted"}
	if err := db.Create(&passkey).Error; err != nil {
		t.Fatal(err)
	}
	password := "new-password"
	accounts := NewAccountService(db)
	updated, err := accounts.Update(user.ID, "old-password", nil, &password, "", user.AuthVersion)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AuthVersion != user.AuthVersion+1 {
		t.Fatal("version not rotated")
	}
	if _, _, err := tokens.Authenticate(token.Plain); err == nil {
		t.Fatal("old PAT remained valid")
	}
	if _, err := tokens.Create(user.ID, input); !errors.Is(err, ErrAuthVersion) {
		t.Fatal("old request created PAT", err)
	}
	var count int64
	db.Model(&models.PasskeyCredential{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Fatal("ordinary rotation deleted passkey")
	}
	if _, err := NewUserService(db).ResetPassword(user.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	still, _ := accounts.Get(user.ID)
	if still.AuthVersion != updated.AuthVersion {
		t.Fatal("reset affected actor")
	}
	// A failed PAT revoke must roll back both the password and version.
	if err := db.Exec("CREATE TRIGGER fail_revoke BEFORE UPDATE ON personal_access_tokens BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	input.AuthVersion = updated.AuthVersion
	if _, err := tokens.Create(user.ID, input); err != nil {
		t.Fatal(err)
	}
	failed := "must-not-commit"
	if _, err := accounts.Update(user.ID, password, nil, &failed, "", updated.AuthVersion); err == nil {
		t.Fatal("expected failure")
	}
	current, _ := accounts.Get(user.ID)
	if current.AuthVersion != updated.AuthVersion || bcrypt.CompareHashAndPassword([]byte(current.Password), []byte(password)) != nil {
		t.Fatal("partial rotation")
	}
}

func TestRecoveryRequiresLocalCodeAndRevokesPasskeys(t *testing.T) {
	db := newServiceTestDB(t)
	user := createServiceTestUser(t, db, "legacy", "123456", models.RoleAdmin)
	code, err := localsecret.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user).Updates(map[string]any{"password_change_required": true, "bootstrap_code_hash": localsecret.Hash(code)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.PasskeyCredential{UserID: user.ID, Name: "old", CredentialID: "old-key", CredentialData: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	accounts := NewAccountService(db)
	name, password := "newname", "recovered-password"
	if _, err := accounts.Update(user.ID, "123456", &name, nil, code, user.AuthVersion); !errors.Is(err, ErrPasswordChangeRequired) {
		t.Fatal(err)
	}
	for _, invalid := range []string{"", "bad-code"} {
		if _, err := accounts.Update(user.ID, "123456", nil, &password, invalid, user.AuthVersion); !errors.Is(err, ErrInvalidBootstrapCode) {
			t.Fatal(err)
		}
	}
	updated, err := accounts.Update(user.ID, "123456", nil, &password, code, user.AuthVersion)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PasswordChangeRequired || updated.BootstrapCodeHash != nil {
		t.Fatal("challenge not consumed")
	}
	var count int64
	db.Model(&models.PasskeyCredential{}).Count(&count)
	if count != 0 {
		t.Fatal("old passkey survived")
	}
	if _, err := accounts.Update(user.ID, "123456", nil, &password, code, user.AuthVersion); err == nil {
		t.Fatal("replay accepted")
	}
}
