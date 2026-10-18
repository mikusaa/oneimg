package images

import (
	"errors"
	"testing"
)

func TestValidatePathTemplate(t *testing.T) {
	valid := []string{
		"",
		"/uploads/{year}/{month}",
		"uploads/{year}/{moon}",
		"/uploads/{role}/{year}/{month}/{day}",
		"/img/{year}",
		"/uploads/图片/{random}",
		"/uploads//{year}",
	}
	for _, value := range valid {
		if err := ValidatePathTemplate(value); err != nil {
			t.Errorf("ValidatePathTemplate(%q) = %v, want nil", value, err)
		}
	}
	invalid := []string{
		"/uploads/{yaer}",
		"/uploads/{Year}",
		"/uploads/{}",
		"/uploads/{year",
		"/uploads/year}",
		"/uploads/{{year}}",
		"/uploads/ {year}",
		"/uploads/\t{year}",
		`\uploads\{year}`,
		"/uploads/../data",
		"../uploads",
		"/uploads/./{year}",
		"/uploads/{year}/..",
		"/uploads/a\x00b",
	}
	for _, value := range invalid {
		err := ValidatePathTemplate(value)
		if !errors.Is(err, ErrTemplateInvalid) {
			t.Errorf("ValidatePathTemplate(%q) = %v, want ErrTemplateInvalid", value, err)
		}
	}
}

func TestValidateNameTemplate(t *testing.T) {
	valid := []string{"", "{random}", "{uuid}", "{filename}-{year}{month}{day}", "blog {second}", "a/b:{hour}", "封面-{minute}"}
	for _, value := range valid {
		if err := ValidateNameTemplate(value); err != nil {
			t.Errorf("ValidateNameTemplate(%q) = %v, want nil", value, err)
		}
	}
	invalid := []string{"{date}", "{Filename}", "{year", "year}", "a​b", "a\nb"}
	for _, value := range invalid {
		if err := ValidateNameTemplate(value); !errors.Is(err, ErrTemplateInvalid) {
			t.Errorf("ValidateNameTemplate(%q) = %v, want ErrTemplateInvalid", value, err)
		}
	}
}
