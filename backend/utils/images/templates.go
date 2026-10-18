package images

import (
	"errors"
	"strings"
	"unicode"
)

// ErrTemplateInvalid 表示存储路径或文件名模板不合法；具体原因见错误信息。
var ErrTemplateInvalid = errors.New("invalid name template")

// templateVariables 与 ReplaceMagicVariables 支持的变量保持一致；moon 为 month 的旧写法。
var templateVariables = map[string]bool{
	"year": true, "month": true, "moon": true, "day": true, "hour": true, "minute": true, "second": true,
	"role": true, "random": true, "uuid": true, "filename": true,
}

func templateError(message string) error {
	return &templateValidationError{message: message}
}

type templateValidationError struct{ message string }

func (e *templateValidationError) Error() string { return e.message }
func (e *templateValidationError) Unwrap() error { return ErrTemplateInvalid }

// checkTemplateVariables 要求花括号成对，且只使用已知变量（区分大小写）。
func checkTemplateVariables(value string) error {
	rest := value
	for {
		open := strings.IndexAny(rest, "{}")
		if open < 0 {
			return nil
		}
		if rest[open] == '}' {
			return templateError("花括号没有成对")
		}
		end := strings.IndexAny(rest[open+1:], "{}")
		if end < 0 || rest[open+1+end] == '{' {
			return templateError("花括号没有成对")
		}
		name := rest[open+1 : open+1+end]
		if !templateVariables[name] {
			return templateError("不支持的变量 {" + name + "}")
		}
		rest = rest[open+1+end+1:]
	}
}

func hasControlRune(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return true
		}
	}
	return false
}

// ValidatePathTemplate 校验存储路径模板。空值表示使用默认路径。
func ValidatePathTemplate(value string) error {
	if value == "" {
		return nil
	}
	if hasControlRune(value) {
		return templateError("存储路径不能包含控制字符")
	}
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return templateError("存储路径不能包含空格")
	}
	if strings.Contains(value, `\`) {
		return templateError(`存储路径的目录之间要用 /，不能用 \`)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return templateError("存储路径不能包含 . 或 .. 目录")
		}
	}
	return checkTemplateVariables(value)
}

// ValidateNameTemplate 校验文件名模板。/ 等特殊字符会在生成时由 SafeName 替换，这里不拒绝。
func ValidateNameTemplate(value string) error {
	if value == "" {
		return nil
	}
	if hasControlRune(value) {
		return templateError("文件名模板不能包含控制字符")
	}
	return checkTemplateVariables(value)
}
