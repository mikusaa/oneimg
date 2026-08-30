package routes

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

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
		{http.MethodGet, "/api/v1/me/tokens"}, {http.MethodPost, "/api/v1/me/tokens"},
		{http.MethodPost, "/api/v1/me/tokens/1/revoke"},
		{http.MethodGet, "/api/v1/upload-options"},
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
	created, err := system.Services.Tokens.Create(admin.ID, services.CreateTokenInput{Name: "users", Scopes: []string{"users:read"}, CurrentPassword: "correct-password"})
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
	created, err := system.Services.Tokens.Create(user.ID, services.CreateTokenInput{Name: "self", Scopes: []string{"images:read"}, CurrentPassword: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", strings.NewReader(`{"name":"nested","scopes":["images:read"],"current_password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+created.Plain)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
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
