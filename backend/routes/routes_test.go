package routes

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"oneimg/backend/app"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/passkeys"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var testFrontend fs.FS = fstest.MapFS{
	"frontend/dist/index.html":    &fstest.MapFile{Data: []byte("<!doctype html><title>oneimg</title>")},
	"frontend/dist/assets/app.js": &fstest.MapFile{Data: []byte("void 0")},
	"api/openapi.yaml":            &fstest.MapFile{Data: []byte("openapi: 3.1.0")},
}

func setupTestRouter(t *testing.T, cfg *config.Config) (*gin.Engine, *app.System) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if cfg.AppURL == "" {
		cfg.AppURL = "http://localhost:8080"
	}
	config.App = cfg
	database.InitDB(cfg)
	db := database.GetDB()
	if err := db.DB.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}
	system := &app.System{Config: cfg, Database: db, Services: services.New(db, cfg)}
	return SetupRoutes(testFrontend, system), system
}

func TestOpenAPIDocuments(t *testing.T) {
	cfg := &config.Config{
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
	}
	router, _ := setupTestRouter(t, cfg)

	for _, test := range []struct {
		path        string
		contentType string
	}{
		{path: "/api/openapi.yaml", contentType: "application/yaml"},
		{path: "/api/openapi.json", contentType: "application/json"},
	} {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if !strings.HasPrefix(recorder.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
			}
		})
	}

	jsonDocument := httptest.NewRecorder()
	router.ServeHTTP(jsonDocument, httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil))
	var document map[string]any
	if err := json.Unmarshal(jsonDocument.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("openapi version = %#v", document["openapi"])
	}

	docs := httptest.NewRecorder()
	router.ServeHTTP(docs, httptest.NewRequest(http.MethodGet, "/api/docs", nil))
	if docs.Code != http.StatusOK || !strings.Contains(docs.Body.String(), "/api/openapi.yaml") {
		t.Fatalf("docs status = %d, body = %s", docs.Code, docs.Body.String())
	}
}

func TestPublicWriteMethodsAndForwardedIP(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test", ConfigSecret: "test"}
		if trusted {
			cfg.TrustedProxies = []string{"192.0.2.1"}
		}
		router, _ := setupTestRouter(t, cfg)
		for _, method := range []string{"DELETE", "PUT", "POST", "PATCH"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(method, "/uploads/missing.png", nil))
			if w.Code != 405 || w.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("%s: %d %v", method, w.Code, w.Header())
			}
		}
		router.GET("/client-ip", func(c *gin.Context) { c.String(200, c.ClientIP()) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/client-ip", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Forwarded-For", "198.51.100.9")
		router.ServeHTTP(w, req)
		expected := "192.0.2.1"
		if trusted {
			expected = "198.51.100.9"
		}
		if w.Body.String() != expected {
			t.Fatal(w.Body.String())
		}
	}
}

func TestExternalAuthenticationRoutesAreRemoved(t *testing.T) {
	cfg := &config.Config{
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
	}
	router, _ := setupTestRouter(t, cfg)

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
	cfg := &config.Config{
		AppURL:        "http://localhost:8080",
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
		PasskeyRPName: "OneImg",
	}
	config.App = cfg
	if err := passkeys.Init(cfg); err != nil {
		t.Fatal(err)
	}
	router, _ := setupTestRouter(t, cfg)

	begin := httptest.NewRecorder()
	router.ServeHTTP(begin, httptest.NewRequest(http.MethodPost, "/api/v1/auth/passkeys/login/options", nil))
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
	router.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/api/v1/me/passkeys", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("protected Passkey route status = %d", protected.Code)
	}
}

func TestLegacyRoutesAreRemoved(t *testing.T) {
	cfg := &config.Config{
		SqlitePath:    filepath.Join(t.TempDir(), "oneimg.db"),
		SessionSecret: "test-session-secret",
		ConfigSecret:  "test-config-secret-with-enough-bytes",
	}
	router, _ := setupTestRouter(t, cfg)

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

func TestAPIProblemResponsesIncludeRequestID(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, _ := setupTestRouter(t, cfg)
	for _, test := range []struct {
		method string
		path   string
		status int
	}{{http.MethodGet, "/api/upload", http.StatusNotFound}, {http.MethodPost, "/api/v1/public/config", http.StatusMethodNotAllowed}} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s %s status = %d, body = %s", test.method, test.path, recorder.Code, recorder.Body.String())
		}
		if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
		}
		requestID := recorder.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Fatal("missing X-Request-ID")
		}
		var problem struct {
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
			t.Fatal(err)
		}
		if problem.Status != test.status || problem.RequestID != requestID {
			t.Fatalf("problem = %#v, request ID = %q", problem, requestID)
		}
	}
}

func TestInvalidBearerDoesNotFallBackToSessionAndCSRFIsRequired(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "tester", Password: string(hash), Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	loginBody := `{"username":"tester","password":"correct-password"}`
	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", cfg.AppURL)
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}

	requestWithCookies := func(method, path string, body *strings.Reader) *http.Request {
		request := httptest.NewRequest(method, path, body)
		for _, cookie := range login.Result().Cookies() {
			request.AddCookie(cookie)
		}
		return request
	}
	invalidBearer := requestWithCookies(http.MethodGet, "/api/v1/me", strings.NewReader(""))
	invalidBearer.Header.Set("Authorization", "Bearer invalid")
	invalidBearerRecorder := httptest.NewRecorder()
	router.ServeHTTP(invalidBearerRecorder, invalidBearer)
	if invalidBearerRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("invalid bearer status = %d, body = %s", invalidBearerRecorder.Code, invalidBearerRecorder.Body.String())
	}

	patch := requestWithCookies(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"current_password":"wrong","password":"updated-password"}`))
	patch.Header.Set("Content-Type", "application/json")
	patch.Header.Set("Origin", cfg.AppURL)
	patchRecorder := httptest.NewRecorder()
	router.ServeHTTP(patchRecorder, patch)
	if patchRecorder.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, body = %s", patchRecorder.Code, patchRecorder.Body.String())
	}
}

func TestJSONContentTypeAndBodyLimit(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, _ := setupTestRouter(t, cfg)

	wrongType := httptest.NewRecorder()
	router.ServeHTTP(wrongType, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`)))
	if wrongType.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong content type status = %d, body = %s", wrongType.Code, wrongType.Body.String())
	}

	oversized := httptest.NewRecorder()
	body := strings.NewReader(`{"username":"` + strings.Repeat("a", (1<<20)+1) + `","password":"x"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(oversized, request)
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, body = %s", oversized.Code, oversized.Body.String())
	}
}

func TestUntrustedAPIOriginReturnsProblemDetails(t *testing.T) {
	cfg := &config.Config{AppURL: "https://images.example.com", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, _ := setupTestRouter(t, cfg)
	for _, origin := range []string{"https://attacker.example", "http://localhost:5173"} {
		t.Run(origin, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/public/config", nil)
			request.Header.Set("Origin", origin)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/problem+json") || recorder.Header().Get("X-Request-ID") == "" {
				t.Fatalf("headers = %#v", recorder.Header())
			}
		})
	}
}

func TestLoopbackOriginIsAllowedOnlyForLoopbackAppURL(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, _ := setupTestRouter(t, cfg)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/config", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestV1OperationRoutesAreRegistered(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, _ := setupTestRouter(t, cfg)
	operations := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/public/config"},
		{http.MethodGet, "/api/v1/public/images/random"},
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodPost, "/api/v1/auth/register"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodPost, "/api/v1/auth/passkeys/login/options"},
		{http.MethodPost, "/api/v1/auth/passkeys/login/verify"},
		{http.MethodGet, "/api/v1/me"}, {http.MethodPatch, "/api/v1/me"},
		{http.MethodGet, "/api/v1/me/passkeys"},
		{http.MethodPost, "/api/v1/me/passkeys/registration/options"},
		{http.MethodPost, "/api/v1/me/passkeys/registration/verify"},
		{http.MethodPatch, "/api/v1/me/passkeys/1"},
		{http.MethodPost, "/api/v1/me/passkeys/1/revoke"},
		{http.MethodPut, "/api/v1/me/avatar"}, {http.MethodDelete, "/api/v1/me/avatar"},
		{http.MethodGet, "/api/v1/me/tokens"}, {http.MethodPost, "/api/v1/me/tokens"},
		{http.MethodPost, "/api/v1/me/tokens/1/revoke"},
		{http.MethodGet, "/api/v1/upload-options"},
		{http.MethodGet, "/api/v1/image-storages"},
		{http.MethodGet, "/api/v1/images"}, {http.MethodPost, "/api/v1/images"},
		{http.MethodGet, "/api/v1/images/1"}, {http.MethodDelete, "/api/v1/images/1"},
		{http.MethodPost, "/api/v1/image-imports"},
		{http.MethodPut, "/api/v1/images/1/tags/1"}, {http.MethodDelete, "/api/v1/images/1/tags/1"},
		{http.MethodPatch, "/api/v1/images/tags"},
		{http.MethodGet, "/api/v1/tags"}, {http.MethodPost, "/api/v1/tags"},
		{http.MethodPatch, "/api/v1/tags/1"}, {http.MethodDelete, "/api/v1/tags/1"},
		{http.MethodGet, "/api/v1/storage-buckets"}, {http.MethodPost, "/api/v1/storage-buckets"},
		{http.MethodGet, "/api/v1/storage-buckets/1"}, {http.MethodPatch, "/api/v1/storage-buckets/1"}, {http.MethodDelete, "/api/v1/storage-buckets/1"},
		{http.MethodPost, "/api/v1/storage-connection-tests"},
		{http.MethodGet, "/api/v1/stats/dashboard"}, {http.MethodGet, "/api/v1/stats/images"},
		{http.MethodGet, "/api/v1/users"}, {http.MethodPost, "/api/v1/users"},
		{http.MethodGet, "/api/v1/users/2/avatar"},
		{http.MethodPatch, "/api/v1/users/2"}, {http.MethodDelete, "/api/v1/users/2"},
		{http.MethodPut, "/api/v1/users/2/permissions"},
		{http.MethodPost, "/api/v1/users/2/password-reset"}, {http.MethodPost, "/api/v1/users/2/passkeys/revoke"},
		{http.MethodGet, "/api/v1/settings"}, {http.MethodPatch, "/api/v1/settings"},
	}
	for _, operation := range operations {
		t.Run(operation.method+" "+operation.path, func(t *testing.T) {
			request := httptest.NewRequest(operation.method, operation.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code == http.StatusNotFound || recorder.Code == http.StatusMethodNotAllowed {
				t.Fatalf("route is not registered: status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestBearerScopesAndCurrentUserPermissionsAreIntersected(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	root := models.User{Username: "root", Password: string(hash), Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	admin := models.User{Username: "api-admin", Password: string(hash), Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{"user:list"}, Buckets: []int{}}}
	if err := system.Database.DB.Create(&root).Error; err != nil {
		t.Fatal(err)
	}
	if err := system.Database.DB.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	created, err := system.Services.Tokens.Create(admin.ID, services.CreateTokenInput{AuthVersion: admin.AuthVersion, Name: "users", Scopes: []string{"users:read"}})
	if err != nil {
		t.Fatal(err)
	}

	request := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+created.Plain)
		router.ServeHTTP(recorder, req)
		return recorder
	}
	if recorder := request("/api/v1/users"); recorder.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := request("/api/v1/images"); recorder.Code != http.StatusForbidden {
		t.Fatalf("scope denial status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := system.Database.DB.Model(&admin).Update("role", models.RoleUser).Error; err != nil {
		t.Fatal(err)
	}
	if recorder := request("/api/v1/users"); recorder.Code != http.StatusForbidden {
		t.Fatalf("downgraded user status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := system.Database.DB.Delete(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if recorder := request("/api/v1/users"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("deleted user status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestImageStoragesListsDisabledAndFullStoragesWithReadScope(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	db := system.Database.DB
	user := models.User{Username: "gallery-reader", Password: "unused", Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	disabled := models.Buckets{Name: "gallery-disabled", Type: "webdav", Config: map[string]any{"webdav_pass": "secret"}, Disabled: true}
	full := models.Buckets{Name: "gallery-full", Type: "webdav", Config: map[string]any{}, Capacity: 1, Usage: 1}
	hidden := models.Buckets{Name: "gallery-unused", Type: "webdav", Config: map[string]any{}}
	for _, bucket := range []*models.Buckets{&disabled, &full, &hidden} {
		if err := db.Create(bucket).Error; err != nil {
			t.Fatal(err)
		}
	}
	images := []models.Image{
		{FileName: "disabled.png", Url: "disabled.png", BucketId: disabled.Id, UserId: user.ID},
		{FileName: "full.png", Url: "full.png", BucketId: full.Id, UserId: user.ID},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}
	request := func(scope, path string) *httptest.ResponseRecorder {
		created, err := system.Services.Tokens.Create(user.ID, services.CreateTokenInput{AuthVersion: user.AuthVersion, Name: scope, Scopes: []string{scope}})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+created.Plain)
		router.ServeHTTP(recorder, req)
		return recorder
	}

	response := request("images:read", "/api/v1/image-storages")
	if response.Code != http.StatusOK {
		t.Fatalf("image storages status = %d, body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "config") || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("image storages leaks configuration: %s", response.Body.String())
	}
	var payload struct {
		Data []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			Disabled bool   `json:"disabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 || payload.Data[0].ID != disabled.Id || payload.Data[0].Name != "gallery-disabled" || !payload.Data[0].Disabled || payload.Data[1].ID != full.Id || payload.Data[1].Name != "gallery-full" || payload.Data[1].Disabled {
		t.Fatalf("image storages = %+v", payload.Data)
	}

	// upload-options 仍只表示当前可上传的存储。
	options := request("images:write", "/api/v1/upload-options")
	if options.Code != http.StatusOK {
		t.Fatalf("upload options status = %d, body = %s", options.Code, options.Body.String())
	}
	for _, name := range []string{"gallery-disabled", "gallery-full", "gallery-unused"} {
		if strings.Contains(options.Body.String(), name) {
			t.Fatalf("upload options offers %s: %s", name, options.Body.String())
		}
	}
	if denied := request("images:write", "/api/v1/image-storages"); denied.Code != http.StatusForbidden {
		t.Fatalf("image storages without images:read status = %d, body = %s", denied.Code, denied.Body.String())
	}
}

func TestTagListCountsUsageWithinVisibleImages(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	db := system.Database.DB
	admin := models.User{Username: "tag-admin", Password: "unused", Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
	member := models.User{Username: "tag-member", Password: "unused", Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
	for _, user := range []*models.User{&admin, &member} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	shared := models.Tags{Name: "shared"}
	adminOnly := models.Tags{Name: "admin-only"}
	unused := models.Tags{Name: "unused"}
	for _, tag := range []*models.Tags{&shared, &adminOnly, &unused} {
		if err := db.Create(tag).Error; err != nil {
			t.Fatal(err)
		}
	}
	images := []models.Image{
		{FileName: "a.png", Url: "a.png", BucketId: 1, UserId: admin.ID},
		{FileName: "b.png", Url: "b.png", BucketId: 1, UserId: admin.ID},
		{FileName: "c.png", Url: "c.png", BucketId: 1, UserId: member.ID},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}
	links := []models.ImageToTags{
		{ImageId: images[0].Id, TagId: shared.Id}, {ImageId: images[0].Id, TagId: adminOnly.Id},
		{ImageId: images[1].Id, TagId: adminOnly.Id}, {ImageId: images[2].Id, TagId: shared.Id},
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatal(err)
	}

	counts := func(user models.User) map[string]int64 {
		t.Helper()
		created, err := system.Services.Tokens.Create(user.ID, services.CreateTokenInput{AuthVersion: user.AuthVersion, Name: "tags", Scopes: []string{"tags:read"}})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
		req.Header.Set("Authorization", "Bearer "+created.Plain)
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("tags status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		var payload struct {
			Data []struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Count *int64 `json:"count"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		result := map[string]int64{}
		for _, item := range payload.Data {
			if item.Count == nil {
				t.Fatalf("tag %s has no count: %s", item.Name, recorder.Body.String())
			}
			result[item.Name] = *item.Count
		}
		return result
	}

	// 管理员统计全部文件；普通用户仍能看到全部标签，但只统计自己的文件
	if got := counts(admin); len(got) != 3 || got["shared"] != 2 || got["admin-only"] != 2 || got["unused"] != 0 {
		t.Fatalf("admin tag counts = %v", got)
	}
	if got := counts(member); len(got) != 3 || got["shared"] != 1 || got["admin-only"] != 0 || got["unused"] != 0 {
		t.Fatalf("member tag counts = %v", got)
	}
}

func TestPatchSettingsRejectsInvalidPathTemplateWithFieldProblem(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	admin := models.User{Username: "settings-admin", Password: "unused", Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := system.Database.DB.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	created, err := system.Services.Tokens.Create(admin.ID, services.CreateTokenInput{AuthVersion: admin.AuthVersion, Name: "settings", Scopes: []string{"settings:write"}})
	if err != nil {
		t.Fatal(err)
	}
	patch := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+created.Plain)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		return recorder
	}

	rejected := patch(`{"default_path":"/uploads/../data/{yaer}"}`)
	if rejected.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid path status = %d, body = %s", rejected.Code, rejected.Body.String())
	}
	var problem struct {
		Code   string `json:"code"`
		Errors []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rejected.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "validation_error" || len(problem.Errors) != 1 || problem.Errors[0].Field != "default_path" {
		t.Fatalf("problem = %s", rejected.Body.String())
	}
	if accepted := patch(`{"default_path":"/uploads/{role}/{year}","file_name":"{filename}-{year}"}`); accepted.Code != http.StatusOK {
		t.Fatalf("valid templates status = %d, body = %s", accepted.Code, accepted.Body.String())
	}
	if rejected := patch(`{"file_name":"{date}"}`); rejected.Code != http.StatusUnprocessableEntity || !strings.Contains(rejected.Body.String(), `"field":"file_name"`) {
		t.Fatalf("invalid file name status = %d, body = %s", rejected.Code, rejected.Body.String())
	}
}

// 系统设置页把所有改动放进一次 PATCH：任一字段无效时整体回滚，并用 errors[].field 指出是哪一项
func TestPatchSettingsReportsInvalidFieldAndRollsBack(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	db := system.Database.DB
	disabled := models.Buckets{Name: "paused", Type: "webdav", Config: map[string]any{}, Disabled: true}
	if err := db.Create(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	admin := models.User{Username: "settings-fields", Password: "unused", Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	created, err := system.Services.Tokens.Create(admin.ID, services.CreateTokenInput{AuthVersion: admin.AuthVersion, Name: "settings", Scopes: []string{"settings:read", "settings:write"}})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/settings", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+created.Plain)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		return recorder
	}
	before := call(http.MethodGet, "")
	if before.Code != http.StatusOK {
		t.Fatalf("read settings status = %d, body = %s", before.Code, before.Body.String())
	}

	cases := []struct {
		body, field, code string
	}{
		{`{"max_file_size":10}`, "max_file_size", "invalid"},
		{`{"max_video_file_size":1024}`, "max_video_file_size", "invalid"},
		{`{"allowed_types":"image/png,image/svg+xml"}`, "allowed_types", "invalid"},
		{`{"allowed_types":" "}`, "allowed_types", "invalid"},
		{`{"main_image_quality":101}`, "main_image_quality", "invalid"},
		{`{"skip_compress_formats":""}`, "skip_compress_formats", "invalid"},
		{`{"default_storage":0}`, "default_storage", "invalid"},
		{`{"default_storage":9999}`, "default_storage", "invalid"},
		{`{"default_storage":` + strconv.Itoa(disabled.Id) + `}`, "default_storage", "storage_disabled"},
		{`{"cdn_domain":"ftp://cdn.example.com"}`, "cdn_domain", "invalid"},
		// 有效字段和无效字段放在同一次请求里
		{`{"seo_title":"Changed","registration_enabled":true,"main_image_quality":-1}`, "main_image_quality", "invalid"},
		{`{"seo_icp":"x","file_name":"{date}"}`, "file_name", "invalid"},
	}
	for _, item := range cases {
		recorder := call(http.MethodPatch, item.body)
		var problem struct {
			Errors []struct {
				Field   string `json:"field"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status = %d, body = %s", item.body, recorder.Code, recorder.Body.String())
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
			t.Fatal(err)
		}
		if len(problem.Errors) != 1 || problem.Errors[0].Field != item.field || problem.Errors[0].Code != item.code || problem.Errors[0].Message == "" {
			t.Fatalf("%s problem = %s", item.body, recorder.Body.String())
		}
	}

	after := call(http.MethodGet, "")
	if after.Body.String() != before.Body.String() {
		t.Fatalf("settings changed after rejected patches:\nbefore %s\nafter  %s", before.Body.String(), after.Body.String())
	}
	if ok := call(http.MethodPatch, `{"seo_title":"Changed","main_image_quality":90}`); ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), `"seo_title":"Changed"`) || !strings.Contains(ok.Body.String(), `"main_image_quality":90`) {
		t.Fatalf("valid patch status = %d, body = %s", ok.Code, ok.Body.String())
	}
}

func TestStorageDirectDomainIsPerBucket(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	db := system.Database.DB
	local := models.Buckets{Id: 1, Name: "local", Type: "default", Config: map[string]any{}}
	if err := db.Create(&local).Error; err != nil {
		t.Fatal(err)
	}
	admin := models.User{Username: "domain-admin", Password: "unused", Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	created, err := system.Services.Tokens.Create(admin.ID, services.CreateTokenInput{AuthVersion: admin.AuthVersion, Name: "domain", Scopes: []string{"storage:read", "storage:write", "images:read", "settings:read", "settings:write"}})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+created.Plain)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(recorder, req)
		return recorder
	}
	type storagePayload struct {
		Data struct {
			ID     int            `json:"id"`
			Config map[string]any `json:"config"`
		} `json:"data"`
	}
	createR2 := func(name, domain string) storagePayload {
		t.Helper()
		body := `{"name":"` + name + `","type":"r2","capacity_bytes":1073741824,"config":{"r2_endpoint":"https://r2.example.com","r2_bucket":"` + name + `","r2_access_key":"ak","r2_secret_key":"sk","public_domain":"` + domain + `"}}`
		response := call(http.MethodPost, "/api/v1/storage-buckets", body)
		if response.Code != http.StatusCreated {
			t.Fatalf("create %s status = %d, body = %s", name, response.Code, response.Body.String())
		}
		var payload storagePayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}

	// 系统默认存储仍是本地存储：直链域名不再要求存储是默认存储。
	direct := createR2("direct", "img.example.com/")
	if got := direct.Data.Config["public_domain"]; got != "https://img.example.com" {
		t.Fatalf("normalized domain = %v", got)
	}
	proxied := createR2("proxied", "")
	if _, ok := proxied.Data.Config["public_domain"]; ok {
		t.Fatalf("empty domain should not be stored: %v", proxied.Data.Config)
	}
	images := []models.Image{
		{FileName: "direct.webp", Url: "/uploads/direct.webp", Thumbnail: "/uploads/direct_thumb.webp", BucketId: direct.Data.ID, Storage: "r2", UserId: admin.ID},
		{FileName: "proxied.webp", Url: "/uploads/proxied.webp", BucketId: proxied.Data.ID, Storage: "r2", UserId: admin.ID},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}
	urls := func() map[string][2]string {
		t.Helper()
		response := call(http.MethodGet, "/api/v1/images", "")
		if response.Code != http.StatusOK {
			t.Fatalf("images status = %d, body = %s", response.Code, response.Body.String())
		}
		var payload struct {
			Data []struct {
				FileName  string `json:"file_name"`
				URL       string `json:"url"`
				Thumbnail string `json:"thumbnail"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		result := map[string][2]string{}
		for _, item := range payload.Data {
			result[item.FileName] = [2]string{item.URL, item.Thumbnail}
		}
		return result
	}
	got := urls()
	if got["direct.webp"] != [2]string{"https://img.example.com/uploads/direct.webp", "https://img.example.com/uploads/direct_thumb.webp"} {
		t.Fatalf("direct urls = %v", got["direct.webp"])
	}
	if got["proxied.webp"][0] != "/uploads/proxied.webp" {
		t.Fatalf("proxied url = %v", got["proxied.webp"])
	}

	path := "/api/v1/storage-buckets/" + strconv.Itoa(direct.Data.ID)
	update := func(domain string) *httptest.ResponseRecorder {
		return call(http.MethodPatch, path, `{"name":"direct","type":"r2","capacity_bytes":1073741824,"config":{"public_domain":"`+domain+`"}}`)
	}
	invalid := update("ftp://img.example.com")
	if invalid.Code != http.StatusUnprocessableEntity || !strings.Contains(invalid.Body.String(), `"field":"config.public_domain"`) {
		t.Fatalf("invalid domain status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
	// 只改域名不涉及端点，不需要重新提供密钥；清空后改回经 OneImg 访问。
	if cleared := update(""); cleared.Code != http.StatusOK || strings.Contains(cleared.Body.String(), "public_domain") {
		t.Fatalf("clear domain status = %d, body = %s", cleared.Code, cleared.Body.String())
	}
	if got := urls()["direct.webp"][0]; got != "/uploads/direct.webp" {
		t.Fatalf("cleared url = %q", got)
	}

	webdav := call(http.MethodPost, "/api/v1/storage-buckets", `{"name":"dav","type":"webdav","capacity_bytes":1073741824,"config":{"webdav_url":"https://dav.example.com","webdav_user":"u","webdav_pass":"p","public_domain":"img.example.com"}}`)
	if webdav.Code != http.StatusUnprocessableEntity || !strings.Contains(webdav.Body.String(), `"field":"config.public_domain"`) {
		t.Fatalf("webdav domain status = %d, body = %s", webdav.Code, webdav.Body.String())
	}

	moved := call(http.MethodPatch, "/api/v1/settings", `{"public_image_domain":"img.example.com"}`)
	if moved.Code != http.StatusUnprocessableEntity || !strings.Contains(moved.Body.String(), `"code":"moved"`) {
		t.Fatalf("legacy settings field status = %d, body = %s", moved.Code, moved.Body.String())
	}
	settings := call(http.MethodGet, "/api/v1/settings?groups=upload", "")
	if settings.Code != http.StatusOK || strings.Contains(settings.Body.String(), "public_image_domain") || !strings.Contains(settings.Body.String(), "cdn_domain") {
		t.Fatalf("upload settings status = %d, body = %s", settings.Code, settings.Body.String())
	}
}

func TestDashboardStatsWithSession(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	admin := models.User{Username: "stats-route-admin", Password: string(hash), Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := system.Database.DB.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	images := []models.Image{
		{FileName: "one.png", FileSize: 1024, MimeType: "image/png", UserId: admin.ID, CreatedAt: now},
		{FileName: "two.webp", FileSize: 2048, MimeType: "image/webp", UserId: admin.ID, CreatedAt: now},
	}
	if err := system.Database.DB.Create(&images).Error; err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"stats-route-admin","password":"correct-password"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", cfg.AppURL)
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/stats/dashboard", nil)
	for _, cookie := range login.Result().Cookies() {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			TotalImages  int64 `json:"total_images"`
			TotalSize    int64 `json:"total_size"`
			TodayUploads int64 `json:"today_uploads"`
			MonthUploads int64 `json:"month_uploads"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.TotalImages != 2 || payload.Data.TotalSize != 3072 || payload.Data.TodayUploads != 2 || payload.Data.MonthUploads != 2 {
		t.Fatalf("dashboard response = %#v", payload.Data)
	}
}

func TestBearerCannotManagePersonalTokens(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "token-owner", Password: string(hash), Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	created, err := system.Services.Tokens.Create(user.ID, services.CreateTokenInput{AuthVersion: user.AuthVersion, Name: "self", Scopes: []string{"images:read"}})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", strings.NewReader(`{"name":"nested","scopes":["images:read"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+created.Plain)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestPersonalTokensCanBeCreatedAndRevokedWithoutPassword(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "session-token-owner", Password: string(hash), Role: models.RoleAdmin, Permission: models.Permission{Codes: models.AllPermissionCodes(), Buckets: []int{}}}
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"session-token-owner","password":"correct-password"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", cfg.AppURL)
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}

	cookies := login.Result().Cookies()
	csrf := ""
	for _, cookie := range cookies {
		if cookie.Name == "oneimg-csrf" {
			csrf = cookie.Value
			break
		}
	}
	if csrf == "" {
		t.Fatal("login did not issue a CSRF cookie")
	}
	addSession := func(request *http.Request) {
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		request.Header.Set("Origin", cfg.AppURL)
		request.Header.Set("X-OneImg-CSRF", csrf)
	}

	create := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", strings.NewReader(`{"name":"passwordless","scopes":["images:read"],"expiration_days":90}`))
	createRequest.Header.Set("Content-Type", "application/json")
	addSession(createRequest)
	router.ServeHTTP(create, createRequest)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", create.Code, create.Body.String())
	}
	var created struct {
		Data struct {
			Token  string `json:"token"`
			Record struct {
				ID uint `json:"id"`
			} `json:"record"`
		} `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.Token == "" || created.Data.Record.ID == 0 {
		t.Fatalf("unexpected create response: %#v", created)
	}

	revoke := httptest.NewRecorder()
	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens/"+strconv.FormatUint(uint64(created.Data.Record.ID), 10)+"/revoke", nil)
	addSession(revokeRequest)
	router.ServeHTTP(revoke, revokeRequest)
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", revoke.Code, revoke.Body.String())
	}
	if _, _, err := system.Services.Tokens.Authenticate(created.Data.Token); !errors.Is(err, services.ErrInvalidToken) {
		t.Fatalf("revoked token error = %v", err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, _ := setupTestRouter(t, cfg)
	for attempt := 1; attempt <= 11; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"nobody","password":"wrong"}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if attempt <= 10 && recorder.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d was limited early", attempt)
		}
		if attempt == 11 {
			if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") == "" {
				t.Fatalf("attempt 11 status = %d, retry-after = %q", recorder.Code, recorder.Header().Get("Retry-After"))
			}
		}
	}
}
