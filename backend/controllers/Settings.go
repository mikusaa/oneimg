package controllers

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/models"
	"oneimg/backend/utils/publicurl"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/secureconfig"
	"oneimg/backend/utils/settings"

	"gorm.io/gorm"
)

// 定义请求参数
type UpdateSettingsRequest struct {
	Key   string `json:"key" binding:"required"`
	Value any    `json:"value"`
}

// 自定义查询参数
type GetSettingsRequest struct {
	Keys []string `json:"keys"`
}

func GetSettings(c *gin.Context) {
	var req GetSettingsRequest
	_ = c.ShouldBindJSON(&req)
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}
	responseSettings := secureconfig.SanitizeSettingsForResponse(settingModel)
	allowedGroups := currentSettingPermissions(c)
	if len(allowedGroups) == 0 {
		c.JSON(http.StatusForbidden, result.Error(403, "无权查看系统设置"))
		return
	}
	allowed := make(map[string]struct{}, len(allowedGroups))
	for _, code := range allowedGroups {
		allowed[code] = struct{}{}
	}
	for key := range responseSettings {
		if required := getSettingRequiredPermission(key); required != "" {
			if _, ok := allowed[required]; !ok {
				delete(responseSettings, key)
			}
		}
	}
	responseSettings["setting_permissions"] = allowedGroups
	filtered := filterSettings(responseSettings, req.Keys)
	filtered["setting_permissions"] = allowedGroups

	c.JSON(200, result.Success("ok", filtered))
}

// 返回登录配置
func GetLoginSettings(c *gin.Context) {
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	c.JSON(200, result.Success("ok",
		map[string]any{
			"start_register": settingModel.StartRegister,
		},
	))
}

// 返回网站SEO信息
func GetSEOSettings(c *gin.Context) {
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	c.JSON(200, result.Success("ok",
		map[string]any{
			"seo_title":       settingModel.SEOTitle,
			"seo_description": settingModel.SEODescription,
			"seo_keywords":    settingModel.SEOKeywords,
			"seo_icp":         settingModel.SEOICP,
			"public_security": settingModel.PublicSecurity,
			"seo_icon":        settingModel.SEOicon,
		},
	))
}

func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, result.Error(400, "请求参数错误: "+err.Error()))
		return
	}
	if req.Value == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "设置值不能为空"))
		return
	}
	if required := getSettingRequiredPermission(req.Key); required == "" || !middlewares.HasPermission(c, required) {
		c.JSON(http.StatusForbidden, result.Error(403, "无权修改该设置项"))
		return
	}
	// 查询是否有该设置项
	settingModel, err := settings.GetSettings()
	if err != nil {
		c.JSON(500, result.Error(500, "获取设置失败"))
		return
	}

	// 校验设置数据
	if err := validateSettingData(req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	fieldName, fieldType, err := findSettingsField(reflect.TypeOf(settingModel), req.Key)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	if secureconfig.IsSettingsSensitiveKey(req.Key) {
		if err := updateSensitiveSettingsField(&settingModel, req.Key, req.Value); err != nil {
			c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
			return
		}
	} else if err := updateSettingsField(&settingModel, req.Key, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	// 更新设置项
	db := database.GetDB().DB

	updateColumn, updateValue, err := buildSettingsUpdate(req.Key, req.Value, fieldName, fieldType)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, err.Error()))
		return
	}

	if updateValue == nil && secureconfig.IsSettingsSensitiveKey(req.Key) {
		c.JSON(200, result.Success("无需更新", nil))
		return
	}

	if err := ensureSettingsColumn(db, updateColumn); err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "数据库字段兼容处理失败"))
		log.Println(err)
		return
	}

	if err := db.Model(&settingModel).Update(updateColumn, updateValue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "更新失败"))
		log.Println(err)
		return
	}

	c.JSON(200, result.Success("更新成功", nil))
}

// 辅助函数，筛选设置项

func filterSettings(settingsMap map[string]any, keys []string) map[string]any {
	if len(keys) == 0 {
		return settingsMap
	}

	filteredSettings := make(map[string]any)
	for key, value := range settingsMap {
		if slices.Contains(keys, key) {
			filteredSettings[key] = value
		}
	}
	return filteredSettings
}

func findSettingsField(typ reflect.Type, key string) (string, reflect.Type, error) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == key || field.Name == key {
			return field.Name, field.Type, nil
		}
	}
	return "", nil, fmt.Errorf("设置项 %s 不存在", key)
}

func updateSensitiveSettingsField(settings *models.Settings, key string, value any) error {
	stringValue := strings.TrimSpace(fmt.Sprintf("%v", value))
	switch key {
	case "api_token":
		settings.APIToken = ""
		settings.APITokenHash = stringValue
		return nil
	default:
		return fmt.Errorf("设置项 %s 不支持敏感更新", key)
	}
}

func buildSettingsUpdate(key string, value any, fieldName string, fieldType reflect.Type) (string, any, error) {
	normalizedValue, err := secureconfig.NormalizeSettingValue(key, value)
	if err != nil {
		return "", nil, err
	}

	switch key {
	case "api_token":
		return "api_token_hash", normalizedValue, nil
	case "public_image_domain":
		domain, err := publicurl.NormalizeDomain(fmt.Sprintf("%v", value))
		if err != nil {
			return "", nil, err
		}
		return "public_image_domain", domain, nil
	case "cdn_domain":
		domain, err := publicurl.NormalizeDomain(fmt.Sprintf("%v", value))
		if err != nil {
			return "", nil, err
		}
		return "cdn_domain", domain, nil
	default:
		convertedValue, convertErr := convertValueToTargetType(key, value, fieldType)
		if convertErr != nil {
			return "", nil, convertErr
		}
		return getSettingsColumnName(fieldName), convertedValue, nil
	}
}

func getSettingsColumnName(fieldName string) string {
	settingsType := reflect.TypeOf(models.Settings{})
	if field, ok := settingsType.FieldByName(fieldName); ok {
		gormTag := field.Tag.Get("gorm")
		for _, part := range strings.Split(gormTag, ";") {
			if strings.HasPrefix(part, "column:") {
				return strings.TrimPrefix(part, "column:")
			}
		}
	}
	return fieldName
}

func ensureSettingsColumn(db *gorm.DB, columnName string) error {
	fieldName, err := getSettingsFieldNameByColumn(columnName)
	if err != nil {
		return err
	}
	if db.Migrator().HasColumn(&models.Settings{}, columnName) {
		return nil
	}

	log.Printf("[数据库兼容] settings.%s 字段不存在，尝试创建", columnName)
	if err := db.Migrator().AddColumn(&models.Settings{}, fieldName); err != nil {
		return fmt.Errorf("创建 settings.%s 字段失败: %w", columnName, err)
	}
	return nil
}

func getSettingsFieldNameByColumn(columnName string) (string, error) {
	settingsType := reflect.TypeOf(models.Settings{})
	for i := 0; i < settingsType.NumField(); i++ {
		field := settingsType.Field(i)
		if getSettingsColumnName(field.Name) == columnName || field.Name == columnName {
			return field.Name, nil
		}
	}
	return "", fmt.Errorf("设置字段 %s 不存在", columnName)
}

func updateSettingsField(settings *models.Settings, key string, value any) error {
	// 获取结构体反射值（指针解引用）
	val := reflect.ValueOf(settings).Elem()
	typ := val.Type()

	// 1. 遍历结构体字段，匹配JSON Tag或字段名
	var targetField reflect.Value
	var fieldType reflect.Type
	found := false

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		// 优先匹配 JSON Tag（如 json:"save_webp"）
		jsonTag := field.Tag.Get("json")
		if jsonTag == key || field.Name == key {
			targetField = val.Field(i)
			fieldType = field.Type
			found = true
			break
		}
	}

	// 校验字段是否存在
	if !found {
		return fmt.Errorf("设置项 %s 不存在", key)
	}

	// 2. 校验字段是否可修改（必须是导出字段）
	if !targetField.CanSet() {
		return fmt.Errorf("设置项 %s 不可修改", key)
	}

	// 3. 处理nil值（避免panic）
	if value == nil {
		return fmt.Errorf("设置项 %s 的值不能为空", key)
	}

	// 4. 转换value类型为字段实际类型
	convertedValue, err := convertValueToTargetType(key, value, fieldType)
	if err != nil {
		return err
	}

	valueVal := reflect.ValueOf(convertedValue)

	// 5. 设置字段值
	targetField.Set(valueVal)
	return nil
}

func convertValueToTargetType(key string, value any, targetType reflect.Type) (any, error) {
	valueVal := reflect.ValueOf(value)
	valueType := valueVal.Type()

	// 类型已匹配，直接返回
	if valueType == targetType {
		return value, nil
	}

	// 场景1：反射支持直接转换（如 int→float64、bool→int 等）
	if valueType.ConvertibleTo(targetType) {
		return valueVal.Convert(targetType).Interface(), nil
	}

	// 场景2：反射不支持直接转换，手动处理常见类型解析
	switch targetType.Kind() {
	// 处理 string → float64
	case reflect.Float64:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			floatVal, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 float64，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return floatVal, nil
		}

	// 处理 string → int/int64
	case reflect.Int:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			intVal, err := strconv.Atoi(strVal)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 int，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return intVal, nil
		}
	case reflect.Int64:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			int64Val, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 int64，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return int64Val, nil
		}

	// 处理 string → bool
	case reflect.Bool:
		if valueType.Kind() == reflect.String {
			strVal := valueVal.String()
			boolVal, err := strconv.ParseBool(strVal)
			if err != nil {
				return nil, fmt.Errorf("设置项 %s 类型转换失败，期望 bool，实际 string（值：%s），错误：%v",
					key, strVal, err)
			}
			return boolVal, nil
		}
	case reflect.String:
		// 所有基础类型都可以转为string
		return fmt.Sprintf("%v", value), nil
	}

	// 不支持的转换类型
	return nil, fmt.Errorf("设置项 %s 类型不匹配，期望 %s，实际 %T",
		key, targetType, value)
}

func validateSettingData(key string, value any) error {
	if settingDisabledByPublicDomain(key) {
		setting, err := settings.GetSettings()
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		if publicurl.HasDomain(setting) {
			return fmt.Errorf("已配置图片直链域名，该设置不会生效，请先清空图片直链域名")
		}
	}

	switch key {
	case "public_image_domain", "cdn_domain":
		domain, err := publicurl.NormalizeDomain(fmt.Sprintf("%v", value))
		if err != nil {
			return err
		}
		if domain == "" {
			return nil
		}
		if key == "cdn_domain" {
			return nil
		}
		setting, err := settings.GetSettings()
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		bucketType, err := getBucketTypeByID(setting.DefaultStorage)
		if err != nil {
			return err
		}
		if !publicurl.SupportsStorage(bucketType) {
			return fmt.Errorf("当前默认存储不支持图片直链域名，请先切换到 S3 或 R2 存储")
		}

	case "main_image_quality":
		quality, err := settingValueToInt(value)
		if err != nil {
			return fmt.Errorf("主图压缩质量必须是整数")
		}
		if quality < 0 || quality > 100 {
			return fmt.Errorf("主图压缩质量必须在0-100之间")
		}

	case "skip_compress_formats":
		formats, ok := value.(string)
		if !ok {
			return fmt.Errorf("跳过压缩格式必须是字符串类型，实际类型：%T", value)
		}
		if strings.TrimSpace(formats) == "" {
			return fmt.Errorf("跳过压缩格式不能为空")
		}

	case "default_storage":
		// 检查存储配置是否存在
		id, err := settingValueToInt(value)
		if err != nil {
			return fmt.Errorf("%s", "解析失败: "+err.Error())
		}

		bucketType, err := getBucketTypeByID(id)
		if err != nil {
			return err
		}
		setting, err := settings.GetSettings()
		if err != nil {
			return fmt.Errorf("获取系统配置失败")
		}
		if publicurl.HasDomain(setting) && !publicurl.SupportsStorage(bucketType) {
			return fmt.Errorf("图片直链域名仅支持 S3/R2 存储，请先清空该配置")
		}
	}

	return nil
}

func settingDisabledByPublicDomain(key string) bool {
	switch key {
	case "referer_white_enable",
		"referer_white_list":
		return true
	default:
		return false
	}
}

func getBucketTypeByID(id int) (string, error) {
	db := database.GetDB().DB
	var bucket models.Buckets
	if err := db.First(&bucket, id).Error; err != nil {
		return "", fmt.Errorf("存储桶不存在")
	}
	return bucket.Type, nil
}

func settingValueToInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		num64, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return int(num64), nil
	default:
		return 0, fmt.Errorf("类型错误：%T", value)
	}
}

var settingKeyPermissions = map[string]string{
	"default_storage": "setting:upload", "public_image_domain": "setting:upload", "cdn_domain": "setting:upload",
	"default_path": "setting:upload", "file_name": "setting:upload", "max_file_size": "setting:upload",
	"allowed_types": "setting:upload", "save_original_name": "setting:upload",
	"original_image": "setting:image", "save_webp": "setting:image", "thumbnail": "setting:image",
	"main_image_quality": "setting:image", "skip_compress_formats": "setting:image",
	"start_register":       "setting:security",
	"referer_white_enable": "setting:security", "referer_white_list": "setting:security",
	"start_api": "setting:api", "api_token": "setting:api", "api_token_configured": "setting:api",
	"seo_title": "setting:seo", "seo_description": "setting:seo", "seo_keywords": "setting:seo",
	"seo_icp": "setting:seo", "public_security": "setting:seo", "seo_icon": "setting:seo",
}

func getSettingRequiredPermission(key string) string {
	return settingKeyPermissions[key]
}

func currentSettingPermissions(c *gin.Context) []string {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok || user.Role != models.RoleAdmin {
		return []string{}
	}
	groups := []string{"setting:upload", "setting:image", "setting:security", "setting:api", "setting:seo"}
	if user.ID == models.SuperAdminID {
		return groups
	}
	allowed := make([]string, 0, len(groups))
	for _, code := range groups {
		if user.Permission.HasPermission(code) {
			allowed = append(allowed, code)
		}
	}
	return allowed
}
