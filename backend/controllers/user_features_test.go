package controllers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/secureconfig"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func setupFeatureTestDB(t *testing.T) {
	t.Helper()
	cfg := &config.Config{
		DbType:       "sqlite",
		SqlitePath:   filepath.Join(t.TempDir(), "oneimg.db"),
		ConfigSecret: "test-config-secret-with-enough-bytes",
	}
	config.App = cfg
	database.InitDB(cfg)
}

func performJSONRequest(handler gin.HandlerFunc, method, path string, body any, values map[string]any) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, bytes.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	for key, value := range values {
		context.Set(key, value)
	}
	if strings.HasPrefix(path, "/api/tags/") {
		context.Params = gin.Params{{Key: "id", Value: strings.TrimPrefix(path, "/api/tags/")}}
	}
	if strings.HasPrefix(path, "/api/users/updatePermission/") {
		context.Params = gin.Params{{Key: "id", Value: strings.TrimPrefix(path, "/api/users/updatePermission/")}}
	}
	handler(context)
	return recorder
}

func TestRegisterHonorsSettingAndCreatesNormalUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	if err := db.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}

	body := map[string]any{"username": "new-user", "password": "secret12"}
	if got := performJSONRequest(Register, http.MethodPost, "/api/register", body, nil).Code; got != http.StatusForbidden {
		t.Fatalf("closed registration status = %d", got)
	}
	if err := db.Model(&models.Settings{}).Where("id = 1").Update("start_register", true).Error; err != nil {
		t.Fatal(err)
	}
	if got := performJSONRequest(Register, http.MethodPost, "/api/register", body, nil).Code; got != http.StatusOK {
		t.Fatalf("open registration status = %d", got)
	}

	var user models.User
	if err := db.Where("username = ?", "new-user").First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Role != models.RoleUser || len(user.Permission.Codes) != 0 || len(user.Permission.Buckets) != 0 {
		t.Fatalf("created user = %+v", user)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("secret12")); err != nil {
		t.Fatal("registered password was not hashed correctly")
	}
	if got := performJSONRequest(Register, http.MethodPost, "/api/register", body, nil).Code; got != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d", got)
	}
	if err := db.Model(&models.Settings{}).Where("id = 1").Update("pow_verify", true).Error; err != nil {
		t.Fatal(err)
	}
	powBody := map[string]any{"username": "pow-user", "password": "secret12", "powToken": "invalid"}
	if got := performJSONRequest(Register, http.MethodPost, "/api/register", powBody, nil).Code; got != http.StatusBadRequest {
		t.Fatalf("invalid PoW status = %d", got)
	}
}

func TestNormalUserCanChangePasswordButNotUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "account-user", Password: string(hash), Role: models.RoleUser, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(sessions.Sessions("oneimg-test", cookie.NewStore([]byte("session-secret"))))
	router.POST("/api/account/change", func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Set("user_role", models.RoleUser)
		session := sessions.Default(c)
		session.Set("logged_in", true)
		_ = session.Save()
		ChangeAccountInfo(c)
	})

	requestBody, _ := json.Marshal(map[string]any{"current_password": "old-password", "new_password": "new-password"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/account/change", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("password update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Set-Cookie") == "" {
		t.Fatal("successful account update did not write cleared session cookie")
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("new-password")); err != nil {
		t.Fatal("normal user password was not updated")
	}

	denied := performJSONRequest(ChangeAccountInfo, http.MethodPost, "/api/account/change", map[string]any{
		"current_password": "new-password", "new_username": "renamed-user",
	}, map[string]any{"user_id": user.ID, "user_role": models.RoleUser})
	if denied.Code != http.StatusForbidden {
		t.Fatalf("normal user username update status = %d", denied.Code)
	}
}

func TestUpdateTagValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	if err := db.Create(&models.Tags{Name: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Tags{Name: "used"}).Error; err != nil {
		t.Fatal(err)
	}

	if got := performJSONRequest(UpdateTag, http.MethodPut, "/api/tags/1", map[string]any{"name": " renamed "}, nil).Code; got != http.StatusOK {
		t.Fatalf("rename status = %d", got)
	}
	var tag models.Tags
	if err := db.First(&tag, 1).Error; err != nil || tag.Name != "renamed" {
		t.Fatalf("renamed tag = %+v, err = %v", tag, err)
	}
	if got := performJSONRequest(UpdateTag, http.MethodPut, "/api/tags/1", map[string]any{"name": "used"}, nil).Code; got != http.StatusConflict {
		t.Fatalf("duplicate rename status = %d", got)
	}
	if got := performJSONRequest(UpdateTag, http.MethodPut, "/api/tags/0", map[string]any{"name": "default"}, nil).Code; got != http.StatusForbidden {
		t.Fatalf("default rename status = %d", got)
	}
}

func TestUpdateUserPermissionPreservesOmittedCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	if err := db.Create(&models.Settings{}).Error; err != nil {
		t.Fatal(err)
	}
	users := []models.User{
		{Username: "super", Password: "hash", Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{}, Buckets: []int{}}},
		{Username: "manager", Password: "hash", Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{"tag:update"}, Buckets: []int{}}},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	path := "/api/users/updatePermission/2"
	contextValues := map[string]any{"user_id": models.SuperAdminID, "user_role": models.RoleAdmin}
	if got := performJSONRequest(UpdateUserPermission, http.MethodPost, path, map[string]any{"permission": []int{}}, contextValues).Code; got != http.StatusOK {
		t.Fatalf("legacy permission update status = %d", got)
	}
	var manager models.User
	if err := db.First(&manager, 2).Error; err != nil {
		t.Fatal(err)
	}
	if len(manager.Permission.Codes) != 1 || manager.Permission.Codes[0] != "tag:update" {
		t.Fatalf("omitted codes changed permissions: %v", manager.Permission.Codes)
	}
	if got := performJSONRequest(UpdateUserPermission, http.MethodPost, path, map[string]any{"permission": []int{}, "codes": []string{}}, contextValues).Code; got != http.StatusOK {
		t.Fatalf("explicit empty permission update status = %d", got)
	}
	if err := db.First(&manager, 2).Error; err != nil {
		t.Fatal(err)
	}
	if len(manager.Permission.Codes) != 0 {
		t.Fatalf("explicit empty codes were not applied: %v", manager.Permission.Codes)
	}
}

func TestCheckImageAccessPermissionRoleBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	image := models.Image{UserId: 99, UUID: "owner-uuid", FileName: "a.webp", MD5: md5.Md5("owner-uuida.webp")}

	context.Set("user_id", 99)
	context.Set("user_role", models.RoleGuest)
	context.Set("username", "different-uuid")
	if CheckImageAccessPermission(context, image, "") {
		t.Fatal("guest ID collision bypassed UUID validation")
	}
	context.Set("username", "owner-uuid")
	if !CheckImageAccessPermission(context, image, "") {
		t.Fatal("matching guest ownership was denied")
	}

	admin := &models.User{ID: 5, Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{"image:delete"}}}
	context.Set("user_id", admin.ID)
	context.Set("user_role", admin.Role)
	context.Set("current_user", admin)
	if !CheckImageAccessPermission(context, image, "image:delete") {
		t.Fatal("permitted administrator could not manage another user's image")
	}
	image.UserId = models.SuperAdminID
	if CheckImageAccessPermission(context, image, "image:delete") {
		t.Fatal("non-superadmin managed superadmin image")
	}
}

func TestSettingsResponseKeysHavePermissionGroups(t *testing.T) {
	for key := range secureconfig.SanitizeSettingsForResponse(models.Settings{}) {
		if key == "id" {
			continue
		}
		if getSettingRequiredPermission(key) == "" {
			t.Errorf("settings response key %q has no permission group", key)
		}
	}
}

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		user *models.User
		want int
	}{
		{"superadmin", &models.User{ID: models.SuperAdminID, Role: models.RoleAdmin}, http.StatusOK},
		{"allowed admin", &models.User{ID: 2, Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{"tag:update"}}}, http.StatusOK},
		{"denied admin", &models.User{ID: 2, Role: models.RoleAdmin, Permission: models.Permission{Codes: []string{}}}, http.StatusForbidden},
		{"normal user", &models.User{ID: 3, Role: models.RoleUser}, http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Set("current_user", test.user)
			middlewares.RequirePermission("tag:update")(context)
			if test.want == http.StatusOK {
				if context.IsAborted() {
					t.Fatal("request was unexpectedly aborted")
				}
				return
			}
			if recorder.Code != test.want || !context.IsAborted() {
				t.Fatalf("status = %d, aborted = %v", recorder.Code, context.IsAborted())
			}
		})
	}
}
