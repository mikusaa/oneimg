package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type responseEnvelope struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

type Meta struct {
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

type FieldProblem struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Problem struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Status    int            `json:"status"`
	Detail    string         `json:"detail"`
	Code      string         `json:"code"`
	Instance  string         `json:"instance"`
	RequestID string         `json:"request_id"`
	Errors    []FieldProblem `json:"errors,omitempty"`
}

func writeData(c *gin.Context, status int, data any, meta *Meta) {
	c.JSON(status, responseEnvelope{Data: data, Meta: meta})
}

func writeNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func writeProblem(c *gin.Context, status int, code, detail string, fields ...FieldProblem) {
	title := http.StatusText(status)
	if title == "" {
		title = "Request failed"
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, Problem{
		Type:      fmt.Sprintf("urn:oneimg:problem:%s", code),
		Title:     title,
		Status:    status,
		Detail:    detail,
		Code:      code,
		Instance:  c.Request.URL.Path,
		RequestID: requestID(c),
		Errors:    fields,
	})
}

func requestID(c *gin.Context) string {
	value, _ := c.Get(contextRequestID)
	id, _ := value.(string)
	return id
}
