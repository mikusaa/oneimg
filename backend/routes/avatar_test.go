package routes

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"oneimg/backend/app"
	"oneimg/backend/config"
	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/chai2010/webp"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type avatarUser struct {
	ID        int     `json:"id"`
	AvatarURL *string `json:"avatar_url"`
}

// loginAs 创建用户并返回给请求附加会话 Cookie 和 CSRF 头的函数。
func loginAs(t *testing.T, router *gin.Engine, system *app.System, cfg *config.Config, user models.User) (models.User, func(*http.Request)) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user.Password = string(hash)
	if err := system.Database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	login := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"`+user.Username+`","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", cfg.AppURL)
	router.ServeHTTP(login, request)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	csrf := ""
	for _, cookie := range cookies {
		if cookie.Name == "oneimg-csrf" {
			csrf = cookie.Value
		}
	}
	return user, func(request *http.Request) {
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		request.Header.Set("Origin", cfg.AppURL)
		request.Header.Set("X-OneImg-CSRF", csrf)
	}
}

func avatarRequest(t *testing.T, method, name string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("avatar", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, "/api/v1/me/avatar", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func testAvatarPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func avatarFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func decodeAvatarUser(t *testing.T, recorder *httptest.ResponseRecorder) avatarUser {
	t.Helper()
	var envelope struct {
		Data avatarUser `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s: %v", recorder.Body.String(), err)
	}
	return envelope.Data
}

func TestAvatarUploadReplaceReadAndRemove(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	dir := filepath.Join(filepath.Dir(cfg.SqlitePath), "avatars")
	owner, ownerSession := loginAs(t, router, system, cfg, models.User{Username: "avatar-owner", Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}})

	upload := httptest.NewRecorder()
	request := avatarRequest(t, http.MethodPut, "wide.png", testAvatarPNG(t, 400, 200))
	ownerSession(request)
	router.ServeHTTP(upload, request)
	if upload.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", upload.Code, upload.Body.String())
	}
	first := decodeAvatarUser(t, upload)
	prefix := "/api/v1/users/" + strconv.Itoa(owner.ID) + "/avatar?v="
	if first.AvatarURL == nil || !strings.HasPrefix(*first.AvatarURL, prefix) {
		t.Fatalf("avatar_url = %v", first.AvatarURL)
	}
	if files := avatarFiles(t, dir); len(files) != 1 {
		t.Fatalf("avatar files after upload = %v", files)
	}

	read := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, *first.AvatarURL, nil)
	ownerSession(readRequest)
	router.ServeHTTP(read, readRequest)
	if read.Code != http.StatusOK || read.Header().Get("Content-Type") != "image/webp" || read.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(read.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("read status = %d, headers = %v", read.Code, read.Header())
	}
	cfgImage, err := webp.DecodeConfig(bytes.NewReader(read.Body.Bytes()))
	if err != nil || cfgImage.Width != 256 || cfgImage.Height != 256 {
		t.Fatalf("avatar is not a 256px WebP: %+v, %v", cfgImage, err)
	}

	stale := httptest.NewRecorder()
	staleRequest := httptest.NewRequest(http.MethodGet, prefix+"stale", nil)
	ownerSession(staleRequest)
	router.ServeHTTP(stale, staleRequest)
	if stale.Code != http.StatusOK || stale.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("stale version status = %d, cache = %q", stale.Code, stale.Header().Get("Cache-Control"))
	}

	replace := httptest.NewRecorder()
	replaceRequest := avatarRequest(t, http.MethodPut, "square.png", testAvatarPNG(t, 64, 64))
	ownerSession(replaceRequest)
	router.ServeHTTP(replace, replaceRequest)
	second := decodeAvatarUser(t, replace)
	if replace.Code != http.StatusOK || second.AvatarURL == nil || *second.AvatarURL == *first.AvatarURL {
		t.Fatalf("replace status = %d, avatar_url = %v", replace.Code, second.AvatarURL)
	}
	if files := avatarFiles(t, dir); len(files) != 1 || !strings.Contains(*second.AvatarURL, strings.TrimSuffix(files[0], ".webp")) {
		t.Fatalf("old avatar was not removed: %v", files)
	}

	me := httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	ownerSession(meRequest)
	router.ServeHTTP(me, meRequest)
	if current := decodeAvatarUser(t, me); current.AvatarURL == nil || *current.AvatarURL != *second.AvatarURL {
		t.Fatalf("/me avatar_url = %v", current.AvatarURL)
	}

	remove := httptest.NewRecorder()
	removeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/me/avatar", nil)
	ownerSession(removeRequest)
	router.ServeHTTP(remove, removeRequest)
	if remove.Code != http.StatusOK || decodeAvatarUser(t, remove).AvatarURL != nil {
		t.Fatalf("remove status = %d, body = %s", remove.Code, remove.Body.String())
	}
	if files := avatarFiles(t, dir); len(files) != 0 {
		t.Fatalf("avatar files after remove = %v", files)
	}
	missing := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+strconv.Itoa(owner.ID)+"/avatar", nil)
	ownerSession(missingRequest)
	router.ServeHTTP(missing, missingRequest)
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "avatar_not_found") {
		t.Fatalf("missing avatar status = %d, body = %s", missing.Code, missing.Body.String())
	}
}

func TestAvatarUploadRejectsUnsafeInputAndNonSessionCallers(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	dir := filepath.Join(filepath.Dir(cfg.SqlitePath), "avatars")
	owner, ownerSession := loginAs(t, router, system, cfg, models.User{Username: "avatar-reject", Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}})

	animated, err := os.ReadFile(filepath.Join("..", "utils", "images", "testdata", "animated.webp"))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"vector.svg":    []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"animated.webp": animated,
		"fake.png":      []byte("not an image"),
	} {
		recorder := httptest.NewRecorder()
		request := avatarRequest(t, http.MethodPut, name, data)
		ownerSession(request)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), "unsupported_image_format") {
			t.Fatalf("%s status = %d, body = %s", name, recorder.Code, recorder.Body.String())
		}
	}

	oversized := httptest.NewRecorder()
	oversizedRequest := avatarRequest(t, http.MethodPut, "large.png", bytes.Repeat([]byte{0}, 10<<20+1))
	ownerSession(oversizedRequest)
	router.ServeHTTP(oversized, oversizedRequest)
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, body = %s", oversized.Code, oversized.Body.String())
	}

	noCSRF := httptest.NewRecorder()
	noCSRFRequest := avatarRequest(t, http.MethodPut, "ok.png", testAvatarPNG(t, 32, 32))
	ownerSession(noCSRFRequest)
	noCSRFRequest.Header.Del("X-OneImg-CSRF")
	router.ServeHTTP(noCSRF, noCSRFRequest)
	if noCSRF.Code != http.StatusForbidden || !strings.Contains(noCSRF.Body.String(), "csrf_validation_failed") {
		t.Fatalf("missing CSRF status = %d, body = %s", noCSRF.Code, noCSRF.Body.String())
	}

	token, err := system.Services.Tokens.Create(owner.ID, services.CreateTokenInput{AuthVersion: owner.AuthVersion, Name: "avatar", Scopes: []string{"images:write"}})
	if err != nil {
		t.Fatal(err)
	}
	bearer := httptest.NewRecorder()
	bearerRequest := avatarRequest(t, http.MethodPut, "ok.png", testAvatarPNG(t, 32, 32))
	bearerRequest.Header.Set("Authorization", "Bearer "+token.Plain)
	router.ServeHTTP(bearer, bearerRequest)
	if bearer.Code != http.StatusForbidden || !strings.Contains(bearer.Body.String(), "session_required") {
		t.Fatalf("bearer status = %d, body = %s", bearer.Code, bearer.Body.String())
	}

	if files := avatarFiles(t, dir); len(files) != 0 {
		t.Fatalf("rejected uploads left files: %v", files)
	}
}

func TestAvatarReadPermissionAndUserDeletionCleanup(t *testing.T) {
	cfg := &config.Config{AppURL: "http://localhost:8080", SqlitePath: filepath.Join(t.TempDir(), "oneimg.db"), SessionSecret: "test-session-secret", ConfigSecret: "test-config-secret-with-enough-bytes"}
	router, system := setupTestRouter(t, cfg)
	dir := filepath.Join(filepath.Dir(cfg.SqlitePath), "avatars")
	_, adminSession := loginAs(t, router, system, cfg, models.User{Username: "avatar-admin", Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{"user:list", "user:delete"}, Buckets: []int{}}})
	owner, ownerSession := loginAs(t, router, system, cfg, models.User{Username: "avatar-member", Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}})
	_, otherSession := loginAs(t, router, system, cfg, models.User{Username: "avatar-other", Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}})

	upload := httptest.NewRecorder()
	request := avatarRequest(t, http.MethodPut, "a.png", testAvatarPNG(t, 48, 48))
	ownerSession(request)
	router.ServeHTTP(upload, request)
	avatarURL := decodeAvatarUser(t, upload).AvatarURL
	if upload.Code != http.StatusOK || avatarURL == nil {
		t.Fatalf("upload status = %d, body = %s", upload.Code, upload.Body.String())
	}

	other := httptest.NewRecorder()
	otherRequest := httptest.NewRequest(http.MethodGet, *avatarURL, nil)
	otherSession(otherRequest)
	router.ServeHTTP(other, otherRequest)
	if other.Code != http.StatusForbidden {
		t.Fatalf("other user read status = %d", other.Code)
	}
	anonymous := httptest.NewRecorder()
	router.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, *avatarURL, nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read status = %d", anonymous.Code)
	}

	list := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users?id="+strconv.Itoa(owner.ID), nil)
	adminSession(listRequest)
	router.ServeHTTP(list, listRequest)
	var listed struct {
		Data []avatarUser `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || len(listed.Data) != 1 || listed.Data[0].AvatarURL == nil || *listed.Data[0].AvatarURL != *avatarURL {
		t.Fatalf("user list avatar = %s, %v", list.Body.String(), err)
	}
	admin := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodGet, *avatarURL, nil)
	adminSession(adminRequest)
	router.ServeHTTP(admin, adminRequest)
	if admin.Code != http.StatusOK {
		t.Fatalf("admin read status = %d, body = %s", admin.Code, admin.Body.String())
	}

	remove := httptest.NewRecorder()
	removeRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+strconv.Itoa(owner.ID), nil)
	adminSession(removeRequest)
	router.ServeHTTP(remove, removeRequest)
	if remove.Code != http.StatusNoContent {
		t.Fatalf("delete user status = %d, body = %s", remove.Code, remove.Body.String())
	}
	if files := avatarFiles(t, dir); len(files) != 0 {
		t.Fatalf("deleted user's avatar remains: %v", files)
	}
}
