package passkeys

import (
	"bytes"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/models"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestBuildConfigDefaultsAndValidation(t *testing.T) {
	cfg, err := BuildConfig(&config.Config{AppURL: "http://localhost:8080", ConfigSecret: "test-secret", PasskeyRPName: "OneImg"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPID != "localhost" || len(cfg.RPOrigins) != 1 || cfg.RPOrigins[0] != "http://localhost:8080" {
		t.Fatalf("unexpected config: %#v", cfg)
	}

	if _, err := BuildConfig(&config.Config{AppURL: "http://one.example.com", ConfigSecret: "test-secret"}); err == nil {
		t.Fatal("non-local HTTP APP_URL was accepted")
	}
	if _, err := BuildConfig(&config.Config{
		AppURL:         "https://one.example.com",
		ConfigSecret:   "test-secret",
		PasskeyRPID:    "example.com",
		PasskeyOrigins: []string{"https://other.invalid"},
	}); err == nil {
		t.Fatal("origin outside RP ID was accepted")
	}
	if _, err := BuildConfig(&config.Config{AppURL: "http://localhost:8080"}); err == nil {
		t.Fatal("empty CONFIG_SECRET was accepted")
	}
}

func TestCredentialEncryptionRoundTripAndWrongSecret(t *testing.T) {
	originalConfig := config.App
	t.Cleanup(func() { config.App = originalConfig })
	config.App = &config.Config{ConfigSecret: "passkey-test-secret-one"}

	original := &webauthn.Credential{ID: []byte{1, 2, 3, 4}, PublicKey: []byte{5, 6, 7}}
	credentialID, encrypted, err := EncodeCredential(original)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains([]byte(encrypted), original.PublicKey) {
		t.Fatal("encrypted credential contains plaintext public key")
	}
	stored := models.PasskeyCredential{ID: 1, CredentialID: credentialID, CredentialData: encrypted}
	decoded, err := DecodeCredential(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.ID, original.ID) || !bytes.Equal(decoded.PublicKey, original.PublicKey) {
		t.Fatalf("decoded credential differs: %#v", decoded)
	}

	config.App = &config.Config{ConfigSecret: "passkey-test-secret-two"}
	if _, err := DecodeCredential(stored); err == nil {
		t.Fatal("credential decrypted with the wrong CONFIG_SECRET")
	}
}

func TestUserHandleIsStableAndFixedWidth(t *testing.T) {
	first := UserHandle(42)
	second := UserHandle(42)
	if len(first) != 8 || !bytes.Equal(first, second) {
		t.Fatalf("unexpected user handle: %v", first)
	}
	if bytes.Equal(first, UserHandle(43)) {
		t.Fatal("different users received the same user handle")
	}
}
