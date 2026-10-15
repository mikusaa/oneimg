package images

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"unicode"
)

func NewObjectID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

func SafeName(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if strings.ContainsRune(`/\:"'<>|?*`, r) {
			r = '-'
		}
		if out.Len()+len(string(r)) > 80 {
			break
		}
		out.WriteRune(r)
	}
	name := strings.Trim(out.String(), " .")
	if name == "" {
		return "file"
	}
	return name
}
