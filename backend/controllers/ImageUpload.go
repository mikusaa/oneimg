package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"oneimg/backend/database"
	"oneimg/backend/interfaces"
	"oneimg/backend/models"
	"oneimg/backend/services"
	"oneimg/backend/utils/md5"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/settings"
	"oneimg/backend/utils/telegram"
	"oneimg/backend/utils/uploads"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UploadImages 图片上传主入口
func UploadImages(c *gin.Context) {
	// 初始化上传上下文
	uc := uploads.NewUploadContext(c)

	// 获取数据库连接
	db := database.GetDB()

	var tags []string
	var existingTags []models.Tags
	tagsStr := c.PostForm("tags")
	if tagsStr != "" {
		err := json.Unmarshal([]byte(tagsStr), &tags)
		if err != nil {
			uc.Fail(400, "Tags参数格式错误：%v", err)
			return
		}

		err = db.DB.Where("name IN ?", tags).Find(&existingTags).Error
		if err != nil {
			uc.Fail(500, "Tag查询失败：%v", err)
			return
		}
	}

	// 获取系统配置
	setting, err := settings.GetSettings()
	if err != nil {
		uc.Fail(500, "获取上传配置失败：%v", err)
		return
	}
	if setting.MultiStorageSync {
		uploadImagesMultiStorage(c, setting, existingTags)
		return
	}

	// 获取存储ID
	var bucketID int
	bucketIDStr := c.PostForm("bucket_id")
	if bucketIDStr != "" {
		// 转换为int
		bucketID, err = strconv.Atoi(bucketIDStr)
		if err != nil {
			uc.Fail(400, "存储ID无效")
			return
		}
	} else {
		bucketID = setting.DefaultStorage
	}

	// 检查游客上传
	if isTouristUsername(c.GetString("username")) {
		if setting.DefaultStorage != bucketID {
			uc.Fail(403, "游客不能上传到非默认存储")
			return
		}
	}
	allowed, err := canUseSingleStorageUploadBucket(c, setting, bucketID)
	if err != nil {
		uc.Fail(500, "校验存储权限失败：%v", err)
		return
	}
	if !allowed {
		uc.Fail(403, "无权使用该存储源")
		return
	}

	// 查询存储配置
	var buckets models.Buckets
	if err := db.DB.Where("id = ?", bucketID).First(&buckets).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			uc.Fail(400, "存储配置不存在")
			return
		}
		uc.Fail(500, "存储配置查询失败：%v", err)
		return
	}

	// 解析并校验上传文件
	files, err := uc.ParseAndValidateFiles()
	if err != nil {
		uc.Fail(400, "文件解析失败")
		return
	}

	// 获取文件大小
	var filesize uint64
	if buckets.Id != 1 && buckets.Type != "telegram" {
		for _, file := range files {
			filesize += uint64(file.Size)
		}
		if (buckets.Usage + filesize) >= buckets.Capacity {
			uc.Fail(400, "存储空间已满, 请切换存储")
			return
		}
	}

	// 获取存储上传器
	uploader, err := uc.GetStorageUploader(&setting, &buckets)
	if err != nil {
		uc.Fail(400, "%s", err.Error())
		return
	}

	// 批量处理文件上传（参数匹配接口定义）
	uploadResults := make([]interfaces.ImageUploadResult, 0, len(files))
	successCount := 0

	for _, file := range files {
		fileResult, err := uploader.Upload(c, &setting, &buckets, file)
		if err != nil {
			// 单个文件上传失败不影响其他文件
			uc.Fail(500, "文件[%s]上传失败：%v", file.Filename, err)
			return
		}

		// 保存图片信息到数据库
		imageModel := models.Image{Id: fileResult.ID}
		if !fileResult.Duplicate {
			imageModel = models.Image{
				Url:              fileResult.URL,
				Thumbnail:        fileResult.ThumbnailURL,
				FileName:         fileResult.FileName,
				OriginalFileName: fileResult.OriginalFileName,
				FileSize:         fileResult.FileSize,
				MimeType:         fileResult.MimeType,
				Width:            fileResult.Width,
				Height:           fileResult.Height,
				Storage:          fileResult.Storage,
				BucketId:         bucketID,
				UserId:           c.GetInt("user_id"),
				MD5:              md5.Md5(c.GetString("username") + fileResult.FileName),
				ContentHash:      fileResult.ContentHash,
				UUID:             GetUUID(c),
			}

			if db != nil {
				db.DB.Create(&imageModel)
				fileResult.ID = imageModel.Id
			}
		}

		// 保存文件大小至存储
		if !fileResult.Duplicate && fileResult.Storage != "default" {
			fileSizeUint := uint64(fileResult.FileSize)
			thumbnailSizeUint := uint64(fileResult.ThumbnailSize)
			totalSizeUint := fileSizeUint
			if thumbnailSizeUint > 0 {
				totalSizeUint += thumbnailSizeUint
			}
			result := db.DB.Model(&models.Buckets{}).
				Where("id = ? AND (usage + ? <= capacity OR type IN ('telegram','default') OR capacity = 0)", bucketID, totalSizeUint).
				UpdateColumn("usage", gorm.Expr("usage + ?", totalSizeUint))
			if result.Error != nil {
				log.Printf("更新Usage失败：%v", result.Error)
			}
			if result.RowsAffected == 0 {
				log.Printf("更新Usage无生效，原因：1.桶ID不存在 2.usage+文件大小>容量 3.数据无变更")
			}
		}

		// 上传时关联图片标签
		if len(existingTags) > 0 && imageModel.Id > 0 {
			var imageTagRelations []models.ImageToTags
			for _, tag := range existingTags {
				imageTagRelations = append(imageTagRelations, models.ImageToTags{
					ImageId: imageModel.Id,
					TagId:   tag.Id,
				})
			}

			db.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&imageTagRelations)
		}

		responseResult := *fileResult
		responseResult.URL = applyPublicImageURL(setting, buckets.Type, bucketID, fileResult.URL)
		responseResult.ThumbnailURL = applyThumbnailURL(setting, buckets.Type, bucketID, fileResult.ThumbnailURL)
		uploadResults = append(uploadResults, responseResult)

		if !fileResult.Duplicate && setting.TGNotice {
			placeholderData := telegram.PlaceholderData{
				Username:    c.GetString("username"),
				Date:        time.Now().Format("2006-01-02 15:04:05"),
				Filename:    fileResult.FileName,
				StorageType: buckets.Type,
				URL:         buildImageResponseURL(c, setting, buckets.Type, bucketID, fileResult.URL),
			}

			err := telegram.SendSimpleMsg(
				setting.TGBotToken,   // 机器人Token
				setting.TGReceivers,  // 接收者ChatID
				setting.TGNoticeText, // 模板文本
				placeholderData,      // 占位符数据
			)
			if err != nil {
				log.Println(err)
				// 忽略错误
			}
		}

		successCount++
	}

	if successCount == 0 {
		uc.Fail(500, "所有文件上传失败")
		return
	}

	// 返回上传结果
	uc.Success("上传成功", map[string]any{
		"files": uploadResults,
		"count": successCount,
	})
}

func uploadImagesMultiStorage(c *gin.Context, setting models.Settings, existingTags []models.Tags) {
	uc := uploads.NewUploadContext(c)
	db := database.GetDB()

	localBucket, syncBuckets, err := resolveUploadBuckets(c, setting)
	if err != nil {
		uc.Fail(500, "获取用户同步存储源失败：%v", err)
		return
	}

	files, err := uc.ParseAndValidateFiles()
	if err != nil {
		uc.Fail(400, "文件解析失败")
		return
	}

	uploader, err := uc.GetStorageUploader(&setting, &localBucket)
	if err != nil {
		uc.Fail(500, "初始化本机存储失败：%s", err.Error())
		return
	}

	uploadResults := make([]interfaces.ImageUploadResult, 0, len(files))
	successCount := 0

	for _, file := range files {
		fileResult, err := uploader.Upload(c, &setting, &localBucket, file)
		if err != nil {
			uc.Fail(500, "文件[%s]保存到本机失败：%v", file.Filename, err)
			return
		}

		imageModel := models.Image{Id: fileResult.ID}
		if !fileResult.Duplicate {
			imageModel = models.Image{
				Url:              fileResult.URL,
				Thumbnail:        fileResult.ThumbnailURL,
				FileName:         fileResult.FileName,
				OriginalFileName: fileResult.OriginalFileName,
				FileSize:         fileResult.FileSize,
				MimeType:         fileResult.MimeType,
				Width:            fileResult.Width,
				Height:           fileResult.Height,
				Storage:          fileResult.Storage,
				BucketId:         localBucket.Id,
				UserId:           c.GetInt("user_id"),
				MD5:              md5.Md5(c.GetString("username") + fileResult.FileName),
				ContentHash:      fileResult.ContentHash,
				UUID:             GetUUID(c),
			}

			now := time.Now()
			err = db.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Create(&imageModel).Error; err != nil {
					return err
				}
				fileResult.ID = imageModel.Id

				localStatus := models.ImageStorage{
					ImageID:       imageModel.Id,
					BucketID:      localBucket.Id,
					Storage:       localBucket.Type,
					Status:        models.ImageStorageStatusSuccess,
					URL:           fileResult.URL,
					Thumbnail:     fileResult.ThumbnailURL,
					FileSize:      fileResult.FileSize,
					ThumbnailSize: fileResult.ThumbnailSize,
					SyncedAt:      &now,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&localStatus).Error; err != nil {
					return err
				}

				for _, bucket := range syncBuckets {
					storageStatus := models.ImageStorage{
						ImageID:       imageModel.Id,
						BucketID:      bucket.Id,
						Storage:       bucket.Type,
						Status:        models.ImageStorageStatusPending,
						URL:           fileResult.URL,
						Thumbnail:     fileResult.ThumbnailURL,
						FileSize:      fileResult.FileSize,
						ThumbnailSize: fileResult.ThumbnailSize,
					}
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&storageStatus).Error; err != nil {
						return err
					}
				}

				if len(existingTags) > 0 {
					imageTagRelations := make([]models.ImageToTags, 0, len(existingTags))
					for _, tag := range existingTags {
						imageTagRelations = append(imageTagRelations, models.ImageToTags{
							ImageId: imageModel.Id,
							TagId:   tag.Id,
						})
					}
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&imageTagRelations).Error; err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				cleanupLocalUpload(imageModel)
				uc.Fail(500, "保存文件记录失败：%v", err)
				return
			}
			services.WakeStorageSyncWorker()
		}

		if fileResult.Duplicate && len(existingTags) > 0 && imageModel.Id > 0 {
			var imageTagRelations []models.ImageToTags
			for _, tag := range existingTags {
				imageTagRelations = append(imageTagRelations, models.ImageToTags{
					ImageId: imageModel.Id,
					TagId:   tag.Id,
				})
			}
			db.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&imageTagRelations)
		}

		responseResult := *fileResult
		responseResult.ID = imageModel.Id
		responseResult.URL = applyPublicImageURL(setting, localBucket.Type, localBucket.Id, fileResult.URL)
		responseResult.ThumbnailURL = applyThumbnailURL(setting, localBucket.Type, localBucket.Id, fileResult.ThumbnailURL)
		uploadResults = append(uploadResults, responseResult)

		if !fileResult.Duplicate && setting.TGNotice {
			placeholderData := telegram.PlaceholderData{
				Username:    c.GetString("username"),
				Date:        time.Now().Format("2006-01-02 15:04:05"),
				Filename:    fileResult.FileName,
				StorageType: localBucket.Type,
				URL:         buildImageResponseURL(c, setting, localBucket.Type, localBucket.Id, fileResult.URL),
			}
			if err := telegram.SendSimpleMsg(setting.TGBotToken, setting.TGReceivers, setting.TGNoticeText, placeholderData); err != nil {
				log.Println(err)
			}
		}

		successCount++
	}

	if successCount == 0 {
		uc.Fail(500, "所有文件上传失败")
		return
	}

	uc.Success("上传成功", map[string]any{
		"files": uploadResults,
		"count": successCount,
	})
}

// UploadImage 单文件上传
func UploadImage(c *gin.Context) {
	UploadImages(c)
}

func AddImageTag(c *gin.Context) {
	// 获取请求参数
	type TagRequest struct {
		Id  int    `json:"id"`  // 图片ID
		Tag string `json:"tag"` // 标签ID（前端传字符串，后端转换）
	}

	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	// 参数非空校验
	if req.Id <= 0 || req.Tag == "" {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	// 转换并校验图片ID
	tagId, err := strconv.Atoi(req.Tag)
	if err != nil || tagId <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "标签ID无效"))
		return
	}
	imageId := req.Id

	// 获取数据库连接
	db := database.GetDB().DB

	// 查询图片是否存在
	var image models.Image
	if err := db.Where("id = ?", imageId).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "图片不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片失败："+err.Error()))
		}
		return
	}
	if !CheckImageAccessPermission(c, image) {
		c.JSON(http.StatusForbidden, result.Error(403, "无权操作该图片"))
		return
	}

	// 查询标签是否存在
	var tag models.Tags
	if err := db.Where("id = ?", tagId).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "标签不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败："+err.Error()))
		}
		return
	}

	// 查询图片是否已经添加过该标签
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err == nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "图片已添加过该标签"))
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查标签关联失败："+err.Error()))
		return
	}

	// 添加图片标签关联
	if err := db.Create(&models.ImageToTags{
		ImageId: imageId,
		TagId:   tagId,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "添加标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("标签添加成功", nil))
}

func DeleteImageTag(c *gin.Context) {
	// 获取请求参数
	type TagRequest struct {
		Id  int `json:"id"`  // 图片ID
		Tag int `json:"tag"` // 标签ID
	}

	var req TagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	// 参数非空校验
	if req.Id <= 0 || req.Tag <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	// 转换并校验图片ID
	tagId := req.Tag
	imageId := req.Id

	// 获取数据库连接
	db := database.GetDB().DB

	// 查询图片是否存在
	var image models.Image
	if err := db.Where("id = ?", imageId).First(&image).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "图片不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片失败："+err.Error()))
		}
		return
	}
	if !CheckImageAccessPermission(c, image) {
		c.JSON(http.StatusForbidden, result.Error(403, "无权操作该图片"))
		return
	}

	// 检查标签是否已经添加过该图片
	var imageTag models.ImageToTags
	if err := db.Where("image_id = ? AND tag_id = ?", imageId, tagId).First(&imageTag).Error; err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "关联不存在"))
		return
	}

	// 删除图片标签关联
	if err := db.Delete(&imageTag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "删除标签失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, result.Success("标签删除成功", nil))
}

// 批量删除tag
func DeleteImageTags(c *gin.Context) {
	type Request struct {
		Images []int  `json:"image_ids"`
		Tag    string `json:"tag_id"`
	}

	var req Request

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	tagID, err := strconv.Atoi(req.Tag)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "tag_id必须是有效数字"))
		return
	}

	if len(req.Images) <= 0 || tagID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	// 直接执行删除操作，不返回结果
	db := database.GetDB().DB
	if status, message, ok := ensureImageBatchAccess(c, db, req.Images); !ok {
		c.JSON(status, result.Error(status, message))
		return
	}

	for _, imageId := range req.Images {
		db.Where("image_id = ? AND tag_id = ?", imageId, tagID).Delete(&models.ImageToTags{})
	}

	c.JSON(http.StatusOK, result.Success("批量删除标签成功", nil))
}

// 批量添加tag
func AddImageTags(c *gin.Context) {
	type Request struct {
		Images []int  `json:"image_ids"`
		Tag    string `json:"tag_id"`
	}

	var req Request

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数解析失败："+err.Error()))
		return
	}

	tagID, err := strconv.Atoi(req.Tag)
	if err != nil {
		c.JSON(http.StatusBadRequest, result.Error(400, "tag_id必须是有效数字"))
		return
	}

	if len(req.Images) <= 0 || tagID <= 0 {
		c.JSON(http.StatusBadRequest, result.Error(400, "参数错误"))
		return
	}

	db := database.GetDB().DB

	// 检查标签是否存在
	var tag models.Tags
	if err := db.Where("id = ?", tagID).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, result.Error(400, "标签不存在"))
		} else {
			c.JSON(http.StatusInternalServerError, result.Error(500, "查询标签失败："+err.Error()))
		}
		return
	}

	var existImageIDs []int
	if err := db.Model(&models.Image{}).Where("id IN (?)", req.Images).Pluck("id", &existImageIDs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "查询图片列表失败："+err.Error()))
		return
	}
	if status, message, ok := ensureImageBatchAccess(c, db, req.Images); !ok {
		c.JSON(status, result.Error(status, message))
		return
	}

	var existRelations []int
	if err := db.Model(&models.ImageToTags{}).
		Where("image_id IN (?) AND tag_id = ?", req.Images, tagID).
		Pluck("image_id", &existRelations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "检查标签关联失败："+err.Error()))
		return
	}

	var insertData []models.ImageToTags
	existRelationMap := make(map[int]bool)
	for _, id := range existRelations {
		existRelationMap[id] = true
	}
	for _, imageID := range req.Images {
		if existRelationMap[imageID] {
			continue
		}
		insertData = append(insertData, models.ImageToTags{
			ImageId: imageID,
			TagId:   tagID,
		})
	}

	if len(insertData) > 0 {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.CreateInBatches(&insertData, 100).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, result.Error(500, "批量添加标签失败："+err.Error()))
			return
		}
	} else {
		c.JSON(http.StatusOK, result.Success("没有需要添加的标签", nil))
		return
	}

	c.JSON(http.StatusOK, result.Success("批量添加标签成功", nil))
}

func ensureImageBatchAccess(c *gin.Context, db *gorm.DB, imageIDs []int) (int, string, bool) {
	uniqueIDs := make([]int, 0, len(imageIDs))
	seen := make(map[int]struct{}, len(imageIDs))
	for _, imageID := range imageIDs {
		if imageID <= 0 {
			return http.StatusBadRequest, "图片ID无效", false
		}
		if _, ok := seen[imageID]; ok {
			continue
		}
		seen[imageID] = struct{}{}
		uniqueIDs = append(uniqueIDs, imageID)
	}

	var images []models.Image
	if err := db.Where("id IN ?", uniqueIDs).Find(&images).Error; err != nil {
		return http.StatusInternalServerError, "查询图片列表失败：" + err.Error(), false
	}
	if len(images) != len(uniqueIDs) {
		return http.StatusBadRequest, "图片不存在", false
	}
	for _, image := range images {
		if !CheckImageAccessPermission(c, image) {
			return http.StatusForbidden, fmt.Sprintf("无权操作图片 %d", image.Id), false
		}
	}
	return http.StatusOK, "", true
}

// 获取上传配置
func GetUploadConfig(c *gin.Context) {
	var tags []models.Tags
	var buckets []models.Buckets

	db := database.GetDB().DB
	query := db.Model(&models.Tags{})
	// 获取标签列表
	if err := query.Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取标签列表失败"))
		return
	}

	setting, _ := settings.GetSettings()
	allowedBuckets, bucketErr := resolveSingleStorageUploadBuckets(c, setting)
	if bucketErr != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储桶权限失败"))
		return
	}
	bucketIDs := make([]int, 0, len(allowedBuckets))
	for _, bucket := range allowedBuckets {
		bucketIDs = append(bucketIDs, bucket.Id)
	}
	if len(bucketIDs) == 0 {
		buckets = []models.Buckets{}
	} else if err := db.Model(&models.Buckets{}).Where("id IN ?", bucketIDs).Find(&buckets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, result.Error(500, "获取存储桶列表失败"))
		return
	}

	var bucketRes []map[string]any
	for _, bucket := range buckets {
		// 过滤已满的存储桶
		if bucket.Capacity > 0 && bucket.Usage >= bucket.Capacity {
			continue
		}
		res := map[string]any{
			"id":   bucket.Id,
			"name": bucket.Name,
			"type": bucket.Type,
		}
		bucketRes = append(bucketRes, res)
	}

	// 构造返回参数
	config := map[string]any{
		"buckets":        bucketRes,
		"tags":           tags,
		"default_bucket": setting.DefaultStorage,
	}

	c.JSON(http.StatusOK, result.Success("ok", config))
}

// 通过URL上传图片
func UploadImagesByURL(c *gin.Context) {
	uc := uploads.NewUploadContext(c)
	db := database.GetDB()

	type URLUploadRequest struct {
		Urls     string `json:"url" binding:"required"`
		Tag      string `json:"tag_id"`
		BucketID string `json:"bucket_id"`
	}

	var req URLUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		uc.Fail(400, "参数格式错误：%v", err)
		return
	}

	if req.Urls == "" {
		uc.Fail(400, "URL不能为空")
		return
	}
	if req.Tag != "" && req.Tag != "0" {
		var tags models.Tags
		if err := db.DB.Where("id = ?", req.Tag).First(&tags).Error; err != nil {
			uc.Fail(400, "标签不存在")
			return
		}
	}

	setting, err := settings.GetSettings()
	if err != nil {
		uc.Fail(500, "获取上传配置失败：%v", err)
		return
	}

	var bucketID int
	if req.BucketID != "" {
		bucketID, err = strconv.Atoi(req.BucketID)
		if err != nil {
			uc.Fail(400, "存储ID无效")
			return
		}
	} else {
		bucketID = setting.DefaultStorage
	}

	var buckets models.Buckets
	var syncBuckets []models.Buckets
	if setting.MultiStorageSync {
		localBucket, targets, err := resolveUploadBuckets(c, setting)
		if err != nil {
			uc.Fail(500, "获取用户同步存储源失败：%v", err)
			return
		}
		buckets = localBucket
		syncBuckets = targets
		bucketID = localBucket.Id
	} else {
		if isTouristUsername(c.GetString("username")) {
			if setting.DefaultStorage != bucketID {
				uc.Fail(403, "游客不能上传到非默认存储")
				return
			}
		}
		allowed, err := canUseSingleStorageUploadBucket(c, setting, bucketID)
		if err != nil {
			uc.Fail(500, "校验存储权限失败：%v", err)
			return
		}
		if !allowed {
			uc.Fail(403, "无权使用该存储源")
			return
		}
		if err := db.DB.Where("id = ?", bucketID).First(&buckets).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				uc.Fail(400, "存储配置不存在")
				return
			}
			uc.Fail(500, "存储配置查询失败：%v", err)
			return
		}
	}

	// 下载图片
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(req.Urls)
	if err != nil {
		uc.Fail(500, "图片下载失败：%v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		uc.Fail(400, "图片下载失败，远端状态码：%d", resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		uc.Fail(400, "URL不是图片类型")
		return
	}

	fileName := fileNameFromURL(req.Urls)

	fileBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(setting.MaxFileSize)+1))
	if err != nil {
		uc.Fail(500, "读取图片失败：%v", err)
		return
	}
	if len(fileBytes) > setting.MaxFileSize {
		uc.Fail(400, "URL 图片超过文件大小限制")
		return
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="` + fileName + `"`},
		"Content-Type":        {contentType},
	})
	part.Write(fileBytes)
	writer.Close()

	// 伪装请求
	c.Request.Body = io.NopCloser(body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.ContentLength = int64(body.Len())

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		uc.Fail(500, "构造文件失败：%v", err)
		return
	}
	defer file.Close()

	if buckets.Id != 1 && buckets.Type != "telegram" {
		if (buckets.Usage + uint64(header.Size)) > buckets.Capacity {
			uc.Fail(400, "存储空间已满")
			return
		}
	}

	uploader, err := uc.GetStorageUploader(&setting, &buckets)
	if err != nil {
		uc.Fail(400, "获取上传器失败：%s", err.Error())
		return
	}

	fileResult, err := uploader.Upload(c, &setting, &buckets, header)
	if err != nil {
		uc.Fail(500, "上传失败[%s]：%v", fileName, err)
		return
	}

	// 数据库保存
	imageModel := models.Image{
		Id: fileResult.ID,
	}
	if !fileResult.Duplicate {
		imageModel = models.Image{
			Url:              fileResult.URL,
			Thumbnail:        fileResult.ThumbnailURL,
			FileName:         fileResult.FileName,
			OriginalFileName: fileResult.OriginalFileName,
			FileSize:         fileResult.FileSize,
			MimeType:         fileResult.MimeType,
			Width:            fileResult.Width,
			Height:           fileResult.Height,
			Storage:          fileResult.Storage,
			BucketId:         bucketID,
			UserId:           c.GetInt("user_id"),
			MD5:              md5.Md5(c.GetString("username") + fileResult.FileName),
			ContentHash:      fileResult.ContentHash,
			UUID:             GetUUID(c),
		}
		err := db.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&imageModel).Error; err != nil {
				return err
			}
			now := time.Now()
			localStatus := models.ImageStorage{
				ImageID:       imageModel.Id,
				BucketID:      buckets.Id,
				Storage:       buckets.Type,
				Status:        models.ImageStorageStatusSuccess,
				URL:           fileResult.URL,
				Thumbnail:     fileResult.ThumbnailURL,
				FileSize:      fileResult.FileSize,
				ThumbnailSize: fileResult.ThumbnailSize,
				SyncedAt:      &now,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&localStatus).Error; err != nil {
				return err
			}
			for _, bucket := range syncBuckets {
				status := models.ImageStorage{
					ImageID:       imageModel.Id,
					BucketID:      bucket.Id,
					Storage:       bucket.Type,
					Status:        models.ImageStorageStatusPending,
					URL:           fileResult.URL,
					Thumbnail:     fileResult.ThumbnailURL,
					FileSize:      fileResult.FileSize,
					ThumbnailSize: fileResult.ThumbnailSize,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&status).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			cleanupLocalUpload(imageModel)
			uc.Fail(500, "保存文件记录失败：%v", err)
			return
		}
		fileResult.ID = imageModel.Id
		if setting.MultiStorageSync {
			services.WakeStorageSyncWorker()
		}
	}

	// 更新容量
	if !fileResult.Duplicate && fileResult.Storage != "default" {
		fileSizeUint := uint64(fileResult.FileSize)
		thumbnailSizeUint := uint64(fileResult.ThumbnailSize)
		totalSizeUint := fileSizeUint
		if thumbnailSizeUint > 0 {
			totalSizeUint += thumbnailSizeUint
		}
		db.DB.Model(&models.Buckets{}).
			Where("id = ? AND (usage + ? <= capacity OR type IN ('telegram','default') OR capacity = 0)", bucketID, totalSizeUint).
			UpdateColumn("usage", gorm.Expr("usage + ?", totalSizeUint))
	}

	// 标签
	if req.Tag != "" && req.Tag != "0" && imageModel.Id > 0 {
		if tagID, err := strconv.Atoi(req.Tag); err == nil {
			db.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ImageToTags{ImageId: imageModel.Id, TagId: tagID})
		}
	}

	// TG通知
	if !fileResult.Duplicate && setting.TGNotice {
		placeholderData := telegram.PlaceholderData{
			Username:    c.GetString("username"),
			Date:        time.Now().Format("2006-01-02 15:04:05"),
			Filename:    fileResult.FileName,
			StorageType: buckets.Type,
			URL:         buildImageResponseURL(c, setting, buckets.Type, bucketID, fileResult.URL),
		}
		if err := telegram.SendSimpleMsg(setting.TGBotToken, setting.TGReceivers, setting.TGNoticeText, placeholderData); err != nil {
			log.Println(err)
		}
	}

	responseResult := *fileResult
	responseResult.URL = applyPublicImageURL(setting, buckets.Type, bucketID, fileResult.URL)
	responseResult.ThumbnailURL = applyThumbnailURL(setting, buckets.Type, bucketID, fileResult.ThumbnailURL)

	uc.Success("URL 图片上传成功", map[string]any{
		"file": responseResult,
	})
}

func fileNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		name := filepath.Base(parsed.Path)
		if name != "/" && name != "." && name != "" {
			return name
		}
	}
	name := filepath.Base(rawURL)
	if name != "/" && name != "." && name != "" {
		return name
	}
	return fmt.Sprintf("url_image_%d.jpg", time.Now().Unix())
}
