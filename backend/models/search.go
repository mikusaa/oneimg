package models

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

func SearchKey(value string) string {
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(value)))
}

func (u *User) BeforeCreate(*gorm.DB) error {
	u.UsernameSearch = SearchKey(u.Username)
	return nil
}

func (i *Image) BeforeCreate(*gorm.DB) error {
	i.FileNameSearch = SearchKey(i.FileName)
	i.OriginalFileNameSearch = SearchKey(i.OriginalFileName)
	i.URLSearch = SearchKey(i.Url)
	i.ContentHashSearch = SearchKey(i.ContentHash)
	return nil
}

// SearchUpdates keeps map-based updates and their derived values atomic.
func SearchUpdates(values map[string]any) map[string]any {
	for original, derived := range map[string]string{
		"username": "username_search", "file_name": "file_name_search",
		"original_filename": "original_filename_search", "url": "url_search",
		"content_hash": "content_hash_search",
	} {
		if value, ok := values[original].(string); ok {
			values[derived] = SearchKey(value)
		}
	}
	return values
}
