package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/utils/passkeys"

	"github.com/gin-gonic/gin"
)

func TestExternalAuthenticationRoutesAreRemoved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
	}
	config.App = cfg
	database.InitDB(cfg)
	if err := database.GetDB().DB.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}

	frontend := fstest.MapFS{
		"frontend/dist/index.html":    &fstest.MapFile{Data: []byte("<!doctype html><title>oneimg</title>")},
		"frontend/dist/assets/app.js": &fstest.MapFile{Data: []byte("void 0")},
	}
	router := SetupRoutes(frontend)

	for _, path := range []string{
		"/api/auth/oidc/login",
		"/api/auth/oidc/callback",
		"/api/auth/cas/login",
		"/api/auth/cas/callback",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPasskeyRoutesAreWired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		AppURL:        "http://localhost:8080",
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
		PasskeyRPName: "OneImg",
	}
	config.App = cfg
	database.InitDB(cfg)
	if err := database.GetDB().DB.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := passkeys.Init(cfg); err != nil {
		t.Fatal(err)
	}

	frontend := fstest.MapFS{
		"frontend/dist/index.html":    &fstest.MapFile{Data: []byte("<!doctype html><title>oneimg</title>")},
		"frontend/dist/assets/app.js": &fstest.MapFile{Data: []byte("void 0")},
	}
	router := SetupRoutes(frontend)

	begin := httptest.NewRecorder()
	router.ServeHTTP(begin, httptest.NewRequest(http.MethodPost, "/api/passkeys/login/begin", nil))
	if begin.Code != http.StatusOK {
		t.Fatalf("login begin status = %d, body = %s", begin.Code, begin.Body.String())
	}
	var beginResponse struct {
		Data struct {
			Options map[string]any `json:"options"`
		} `json:"data"`
	}
	if err := json.Unmarshal(begin.Body.Bytes(), &beginResponse); err != nil {
		t.Fatal(err)
	}
	if beginResponse.Data.Options["challenge"] == nil || beginResponse.Data.Options["publicKey"] != nil {
		t.Fatalf("unexpected browser options shape: %#v", beginResponse.Data.Options)
	}

	protected := httptest.NewRecorder()
	router.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/api/passkeys", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("protected Passkey route status = %d", protected.Code)
	}
}

func TestLegacyRoutesAreRemoved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
	}
	config.App = cfg
	database.InitDB(cfg)
	if err := database.GetDB().DB.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}

	frontend := fstest.MapFS{
		"frontend/dist/index.html":    &fstest.MapFile{Data: []byte("<!doctype html><title>oneimg</title>")},
		"frontend/dist/assets/app.js": &fstest.MapFile{Data: []byte("void 0")},
	}
	router := SetupRoutes(frontend)

	for _, path := range []string{"/api/upload", "/api/sessions/clear"} {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, path, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
