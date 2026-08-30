package v1

import (
	"net/http"
	"strconv"
	"strings"

	"oneimg/backend/utils/passkeys"

	"github.com/gin-gonic/gin"
)

func itoa(value int) string { return strconv.Itoa(value) }

func passkeysAvailable() bool { return passkeys.Available() }

func parsePositiveID(c *gin.Context, name string) (int, bool) {
	value, err := strconv.Atoi(c.Param(name))
	if err != nil || value <= 0 {
		writeProblem(c, http.StatusBadRequest, "invalid_id", name+" 必须是正整数")
		return 0, false
	}
	return value, true
}

func parsePositiveQuery(c *gin.Context, name string, fallback, maximum int) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || (maximum > 0 && value > maximum) {
		writeProblem(c, http.StatusUnprocessableEntity, "invalid_query_parameter", name+" 参数无效", FieldProblem{Field: name, Code: "invalid", Message: name + " 参数无效"})
		return 0, false
	}
	return value, true
}

func parseCSVPositiveInts(c *gin.Context, name string) ([]int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return []int{}, true
	}
	result := make([]int, 0)
	seen := map[int]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value <= 0 {
			writeProblem(c, http.StatusUnprocessableEntity, "invalid_query_parameter", name+" 只能包含正整数 ID", FieldProblem{Field: name, Code: "invalid", Message: "只能包含正整数 ID"})
			return nil, false
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result, true
}
