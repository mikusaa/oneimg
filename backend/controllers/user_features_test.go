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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFeatureTestDB(t *testing.T) {
	t.Helper()
	cfg := &config.Config{
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

func TestImageTagRequestsAcceptNumericIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	tag := models.Tags{Name: "historical"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	images := []models.Image{
		{Url: "/uploads/old-a.png", FileName: "old-a.png", FileSize: 1, BucketId: 1, UserId: models.SuperAdminID},
		{Url: "/uploads/old-b.png", FileName: "old-b.png", FileSize: 1, BucketId: 1, UserId: models.SuperAdminID},
	}
	if err := db.Create(&images).Error; err != nil {
		t.Fatal(err)
	}
	contextValues := map[string]any{"user_id": models.SuperAdminID, "user_role": models.RoleAdmin}

	single := performJSONRequest(AddImageTag, http.MethodPost, "/api/images/tag", map[string]any{
		"id": images[0].Id, "tag": tag.Id,
	}, contextValues)
	if single.Code != http.StatusOK {
		t.Fatalf("numeric single tag status = %d, body = %s", single.Code, single.Body.String())
	}

	batchBody := map[string]any{"image_ids": []int{images[1].Id}, "tag_id": tag.Id}
	batch := performJSONRequest(AddImageTags, http.MethodPost, "/api/images/tags", batchBody, contextValues)
	if batch.Code != http.StatusOK {
		t.Fatalf("numeric batch tag status = %d, body = %s", batch.Code, batch.Body.String())
	}
	deleted := performJSONRequest(DeleteImageTags, http.MethodDelete, "/api/images/tags", batchBody, contextValues)
	if deleted.Code != http.StatusOK {
		t.Fatalf("numeric batch delete status = %d, body = %s", deleted.Code, deleted.Body.String())
	}

	var firstCount, secondCount int64
	if err := db.Model(&models.ImageToTags{}).Where("image_id = ? AND tag_id = ?", images[0].Id, tag.Id).Count(&firstCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.ImageToTags{}).Where("image_id = ? AND tag_id = ?", images[1].Id, tag.Id).Count(&secondCount).Error; err != nil {
		t.Fatal(err)
	}
	if firstCount != 1 || secondCount != 0 {
		t.Fatalf("unexpected tag relations: first=%d second=%d", firstCount, secondCount)
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
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "oidc") || strings.Contains(lowerKey, "cas") {
			t.Errorf("external authentication setting %q is still exposed", key)
		}
		if strings.Contains(lowerKey, "telegram") || strings.HasPrefix(lowerKey, "tg_") {
			t.Errorf("removed Telegram setting %q is still exposed", key)
		}
		if strings.HasPrefix(lowerKey, "watermark_") {
			t.Errorf("removed watermark setting %q is still exposed", key)
		}
		if key == "id" {
			continue
		}
		if getSettingRequiredPermission(key) == "" {
			t.Errorf("settings response key %q has no permission group", key)
		}
	}
}

func TestLoginSettingsOnlyExposeLocalAuthenticationOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	if err := database.GetDB().DB.Create(&models.Settings{
		PowVerify:     true,
		Tourist:       true,
		StartRegister: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	recorder := performJSONRequest(GetLoginSettings, http.MethodGet, "/api/settings/login", nil, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login settings status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 3 || response.Data["pow_verify"] != true || response.Data["tourist"] != true || response.Data["start_register"] != true {
		t.Fatalf("unexpected login settings: %#v", response.Data)
	}
	for key := range response.Data {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "oidc") || strings.Contains(lowerKey, "cas") {
			t.Fatalf("external authentication setting leaked: %s", key)
		}
	}
}

func TestDeleteUserDoesNotRequireExternalIdentityTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	if db.Migrator().HasTable("external_identities") {
		t.Fatal("new database unexpectedly contains external_identities")
	}
	if db.Migrator().HasTable("external_auth_flows") {
		t.Fatal("new database unexpectedly contains external_auth_flows")
	}
	for _, column := range []string{
		"oidc_enable", "oidc_issuer", "oidc_client_id", "oidc_client_secret", "oidc_redirect_url",
		"oidc_scopes", "oidc_username_claim", "oidc_display_name", "oidc_auto_provision", "oidc_super_admin_username",
		"cas_enable", "cas_server_url", "cas_service_url", "cas_display_name", "cas_auto_provision", "cas_super_admin_username",
	} {
		if db.Migrator().HasColumn("settings", column) {
			t.Fatalf("new database unexpectedly contains settings.%s", column)
		}
	}
	users := []models.User{
		{Username: "super", Password: "hash", Role: models.RoleAdmin},
		{Username: "delete-me", Password: "hash", Role: models.RoleUser},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodDelete, "/api/users/2", nil)
	context.Params = gin.Params{{Key: "id", Value: "2"}}
	context.Set("user_id", models.SuperAdminID)
	DeleteUser(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete user status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Where("id = ?", 2).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("user was not deleted")
	}
}

func TestLegacyExternalAuthSchemaRemainsCompatible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacyDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE settings (id integer PRIMARY KEY AUTOINCREMENT, oidc_enable numeric, oidc_client_secret text, cas_enable numeric)`,
		`INSERT INTO settings (id, oidc_enable, oidc_client_secret, cas_enable) VALUES (1, 0, '', 0)`,
		`CREATE TABLE external_auth_flows (id integer PRIMARY KEY AUTOINCREMENT, state_hash text)`,
		`CREATE TABLE external_identities (id integer PRIMARY KEY AUTOINCREMENT, user_id integer)`,
	}
	for _, statement := range statements {
		if err := legacyDB.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, err := legacyDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{SqlitePath: path, ConfigSecret: "test-config-secret-with-enough-bytes"}
	config.App = cfg
	database.InitDB(cfg)
	migratedDB := database.GetDB().DB
	if !migratedDB.Migrator().HasTable("external_auth_flows") || !migratedDB.Migrator().HasTable("external_identities") {
		t.Fatal("legacy external authentication tables were removed")
	}
	if !migratedDB.Migrator().HasColumn("settings", "oidc_enable") || !migratedDB.Migrator().HasColumn("settings", "cas_enable") {
		t.Fatal("legacy external authentication columns were removed")
	}
	var setting models.Settings
	if err := migratedDB.First(&setting, 1).Error; err != nil {
		t.Fatalf("migrated settings row is unreadable: %v", err)
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

func TestUpdateDefaultStorageCDNNormalizesAndClearsDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupFeatureTestDB(t)
	db := database.GetDB().DB
	setting := models.Settings{CDNDomain: "https://old.example.com"}
	if err := db.Create(&setting).Error; err != nil {
		t.Fatal(err)
	}

	recorder := performJSONRequest(
		UpdateDefaultStorageCDN,
		http.MethodPut,
		"/api/buckets/default/cdn",
		map[string]any{"cdn_domain": " img.example.com/ "},
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update CDN domain status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := db.First(&setting, setting.ID).Error; err != nil {
		t.Fatal(err)
	}
	if setting.CDNDomain != "https://img.example.com" {
		t.Fatalf("normalized CDN domain = %q", setting.CDNDomain)
	}

	recorder = performJSONRequest(
		UpdateDefaultStorageCDN,
		http.MethodPut,
		"/api/buckets/default/cdn",
		map[string]any{"cdn_domain": ""},
		nil,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear CDN domain status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := db.First(&setting, setting.ID).Error; err != nil {
		t.Fatal(err)
	}
	if setting.CDNDomain != "" {
		t.Fatalf("cleared CDN domain = %q", setting.CDNDomain)
	}
}

func TestUpdateDefaultStorageCDNRequiresStorageUpdatePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.PUT(
		"/api/buckets/default/cdn",
		func(c *gin.Context) {
			c.Set("current_user", &models.User{
				ID:         2,
				Role:       models.RoleAdmin,
				Permission: models.Permission{Codes: []string{}},
			})
		},
		middlewares.RequirePermission("storage:update"),
		UpdateDefaultStorageCDN,
	)

	payload, err := json.Marshal(map[string]any{"cdn_domain": "img.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/buckets/default/cdn", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("denied CDN update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
