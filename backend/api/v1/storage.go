package v1

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type storageDTO struct {
	ID            int                   `json:"id"`
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	CapacityBytes uint64                `json:"capacity_bytes"`
	UsageBytes    uint64                `json:"usage_bytes"`
	UsageExact    bool                  `json:"usage_exact"`
	Filesystem    *storageFilesystemDTO `json:"filesystem,omitempty"`
	Config        map[string]any        `json:"config,omitempty"`
}

type storageFilesystemDTO struct {
	TotalBytes     uint64 `json:"total_bytes"`
	UsedBytes      uint64 `json:"used_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
}

type storageRequest struct {
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	CapacityBytes uint64         `json:"capacity_bytes"`
	Config        map[string]any `json:"config"`
}

func toStorageDTO(item services.StorageBucketSummary) storageDTO {
	result := storageDTO{
		ID: item.Bucket.Id, Name: item.Bucket.Name, Type: item.Bucket.Type,
		CapacityBytes: item.Bucket.Capacity, UsageBytes: item.UsageBytes,
		UsageExact: item.UsageExact, Config: item.Bucket.Config,
	}
	if item.Filesystem != nil {
		result.Filesystem = &storageFilesystemDTO{
			TotalBytes: item.Filesystem.TotalBytes, UsedBytes: item.Filesystem.UsedBytes,
			AvailableBytes: item.Filesystem.AvailableBytes,
		}
	}
	return result
}

func toUploadStorageDTO(item models.Buckets) storageDTO {
	return storageDTO{
		ID: item.Id, Name: item.Name, Type: item.Type,
		CapacityBytes: item.Capacity, UsageBytes: item.Usage, UsageExact: false,
	}
}

func (s *Server) listStorage(c *gin.Context) {
	items, err := s.services.Storage.List()
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "storage_list_failed", "读取存储桶失败")
		return
	}
	result := make([]storageDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toStorageDTO(item))
	}
	writeData(c, http.StatusOK, result, nil)
}

func (s *Server) getStorage(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	item, err := s.services.Storage.Get(id)
	if handleStorageError(c, err) {
		return
	}
	writeData(c, http.StatusOK, toStorageDTO(item), nil)
}

func (s *Server) createStorage(c *gin.Context) {
	var input storageRequest
	if !bindJSON(c, &input) {
		return
	}
	item, err := s.services.Storage.Create(services.StorageInput{Name: input.Name, Type: input.Type, CapacityBytes: input.CapacityBytes, Config: input.Config})
	if handleStorageError(c, err) {
		return
	}
	summary, err := s.services.Storage.Get(item.Id)
	if handleStorageError(c, err) {
		return
	}
	c.Header("Location", "/api/v1/storage-buckets/"+itoa(item.Id))
	writeData(c, http.StatusCreated, toStorageDTO(summary), nil)
}

func (s *Server) updateStorage(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	var input storageRequest
	if !bindJSON(c, &input) {
		return
	}
	item, err := s.services.Storage.Update(id, services.StorageInput{Name: input.Name, Type: input.Type, CapacityBytes: input.CapacityBytes, Config: input.Config})
	if handleStorageError(c, err) {
		return
	}
	summary, err := s.services.Storage.Get(item.Id)
	if handleStorageError(c, err) {
		return
	}
	writeData(c, http.StatusOK, toStorageDTO(summary), nil)
}

func (s *Server) deleteStorage(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	if err := s.services.Storage.Delete(ctx, id); handleStorageError(c, err) {
		return
	}
	writeNoContent(c)
}

func (s *Server) testStorageConnection(c *gin.Context) {
	var input struct {
		ID            *int           `json:"id"`
		Name          string         `json:"name"`
		Type          string         `json:"type"`
		CapacityBytes uint64         `json:"capacity_bytes"`
		Config        map[string]any `json:"config"`
	}
	if !bindJSON(c, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	detail, err := s.services.Storage.TestConnection(ctx, services.StorageInput{Name: input.Name, Type: input.Type, CapacityBytes: input.CapacityBytes, Config: input.Config}, input.ID)
	if errors.Is(err, context.DeadlineExceeded) {
		c.Header("Retry-After", "5")
		writeProblem(c, http.StatusGatewayTimeout, "storage_connection_timeout", "存储连接测试超时")
		return
	}
	if err != nil {
		if errors.Is(err, services.ErrStorageTypeInvalid) || errors.Is(err, services.ErrStorageConfigInvalid) {
			writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "存储配置无效")
			return
		}
		writeProblem(c, http.StatusBadGateway, "storage_connection_failed", "存储连接测试失败")
		return
	}
	writeData(c, http.StatusOK, gin.H{"detail": detail}, nil)
}

func (s *Server) uploadOptions(c *gin.Context) {
	user, _ := currentUser(c)
	setting, buckets, err := s.services.Storage.UploadOptions(*user)
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "upload_options_failed", "读取上传选项失败")
		return
	}
	items := make([]storageDTO, 0, len(buckets))
	for _, bucket := range buckets {
		items = append(items, toUploadStorageDTO(bucket))
	}
	writeData(c, http.StatusOK, gin.H{"max_file_size": setting.MaxFileSize, "allowed_types": strings.Split(setting.AllowedTypes, ","), "max_files": 10, "default_storage": setting.DefaultStorage, "storage_buckets": items}, nil)
}

func handleStorageError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeProblem(c, http.StatusNotFound, "storage_not_found", "存储桶不存在")
	case errors.Is(err, services.ErrDefaultStorage):
		writeProblem(c, http.StatusForbidden, "default_storage_protected", "本地默认存储不能修改或删除")
	case errors.Is(err, services.ErrStorageTypeInvalid), errors.Is(err, services.ErrStorageConfigInvalid), errors.Is(err, services.ErrStorageCapacity):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "存储配置无效")
	case errors.Is(err, services.ErrStorageDelete):
		writeProblem(c, http.StatusBadGateway, "storage_delete_failed", "物理文件删除失败，存储桶和数据库记录已保留")
	case strings.Contains(strings.ToLower(err.Error()), "unique"):
		writeProblem(c, http.StatusConflict, "storage_name_conflict", "存储桶名称已存在")
	default:
		writeProblem(c, http.StatusInternalServerError, "storage_operation_failed", "存储操作失败")
	}
	return true
}
