package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxJSONBodyBytes = 1 << 20

func bindJSON(c *gin.Context, target any) bool {
	if !prepareJSONBody(c) {
		return false
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeProblem(c, http.StatusRequestEntityTooLarge, "request_too_large", "JSON 请求体不能超过 1 MiB")
		} else {
			writeProblem(c, http.StatusBadRequest, "malformed_json", "JSON 请求体格式错误")
		}
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeProblem(c, http.StatusBadRequest, "malformed_json", "JSON 请求体只能包含一个 JSON 值")
		return false
	}
	return true
}

func prepareJSONBody(c *gin.Context) bool {
	mediaType := strings.TrimSpace(strings.SplitN(strings.ToLower(c.GetHeader("Content-Type")), ";", 2)[0])
	if mediaType != "application/json" {
		writeProblem(c, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求体必须使用 application/json")
		return false
	}
	limited := http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBodyBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeProblem(c, http.StatusRequestEntityTooLarge, "request_too_large", "JSON 请求体不能超过 1 MiB")
		} else {
			writeProblem(c, http.StatusBadRequest, "malformed_json", "无法读取 JSON 请求体")
		}
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return true
}
