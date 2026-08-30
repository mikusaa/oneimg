package v1

import (
	"errors"
	"net/http"

	"oneimg/backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) listTags(c *gin.Context) {
	items, err := s.services.Tags.List()
	if err != nil {
		writeProblem(c, http.StatusInternalServerError, "tag_list_failed", "读取标签失败")
		return
	}
	result := make([]tagDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toTagDTO(item))
	}
	writeData(c, http.StatusOK, result, nil)
}

func (s *Server) createTag(c *gin.Context) {
	var input struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &input) {
		return
	}
	item, err := s.services.Tags.Create(input.Name)
	if handleTagError(c, err) {
		return
	}
	c.Header("Location", "/api/v1/tags/"+itoa(item.Id))
	writeData(c, http.StatusCreated, toTagDTO(item), nil)
}

func (s *Server) updateTag(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !bindJSON(c, &input) {
		return
	}
	item, err := s.services.Tags.Update(id, input.Name)
	if handleTagError(c, err) {
		return
	}
	writeData(c, http.StatusOK, toTagDTO(item), nil)
}

func (s *Server) deleteTag(c *gin.Context) {
	id, ok := parsePositiveID(c, "id")
	if !ok {
		return
	}
	if err := s.services.Tags.Delete(id); handleTagError(c, err) {
		return
	}
	writeNoContent(c)
}

func handleTagError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, services.ErrTagNameInvalid):
		writeProblem(c, http.StatusUnprocessableEntity, "validation_error", "标签名称长度必须为 1-10 个字符")
	case errors.Is(err, services.ErrTagConflict):
		writeProblem(c, http.StatusConflict, "tag_name_conflict", "标签名称已存在")
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeProblem(c, http.StatusNotFound, "tag_not_found", "标签不存在")
	default:
		writeProblem(c, http.StatusInternalServerError, "tag_operation_failed", "标签操作失败")
	}
	return true
}
