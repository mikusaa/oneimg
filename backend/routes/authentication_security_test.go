package routes

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"oneimg/backend/config"
	"oneimg/backend/models"
)

func TestTwoSessionsAndRestrictedRoutes(t *testing.T) {
	cfg := &config.Config{SqlitePath: filepath.Join(t.TempDir(), "db"), SessionSecret: "test", ConfigSecret: "test"}
	router, system := setupTestRouter(t, cfg)
	hash, _ := bcrypt.GenerateFromPassword([]byte("current-password"), bcrypt.MinCost)
	user := models.User{Username: "tester", Password: string(hash), Role: models.RoleAdmin, PasswordChangeRequired: true}
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	login := func() []*http.Cookie {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"username":"tester","password":"current-password"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return w.Result().Cookies()
	}
	first, second := login(), login()
	request := func(cookies []*http.Cookie, method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
			if cookie.Name == "oneimg-csrf" {
				req.Header.Set("X-OneImg-CSRF", cookie.Value)
			}
		}
		router.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/api/v1/me/tokens", "/api/v1/me/passkeys", "/api/v1/images", "/api/v1/users", "/api/v1/settings"} {
		w := request(first, "GET", path, "")
		if w.Code != 403 || !strings.Contains(w.Body.String(), "password_change_required") {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	w := request(first, "PATCH", "/api/v1/me", `{"current_password":"current-password","password":"changed-password"}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, cookies := range [][]*http.Cookie{first, second} {
		if w := request(cookies, "GET", "/api/v1/me", ""); w.Code != 401 {
			t.Fatal("old session survived", w.Code)
		}
	}
}
