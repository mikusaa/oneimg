// Package videos identifies the two supported containers without decoding video.
package videos

import (
	"encoding/binary"
	"strings"
)

const HeaderLimit = 64 * 1024

func IsVideo(mime string) bool {
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	return mime == "video/mp4" || mime == "video/webm"
}

// Detect checks bounded container structure, not codecs or full-file integrity.
func Detect(data []byte) string {
	// ISO BMFF: locate an ftyp box, allowing leading free/skip boxes.
	for offset := 0; offset+8 <= len(data); {
		size := uint64(binary.BigEndian.Uint32(data[offset:]))
		kind := string(data[offset+4 : offset+8])
		header := 8
		if size == 1 {
			if offset+16 > len(data) {
				break
			}
			size = binary.BigEndian.Uint64(data[offset+8:])
			header = 16
		}
		if size < uint64(header) || size > uint64(len(data)-offset) {
			break
		}
		end := offset + int(size)
		if kind == "ftyp" {
			payload := data[offset+header : end]
			if len(payload) < 8 || len(payload)%4 != 0 {
				break
			}
			for i := 0; i+4 <= len(payload); i += 4 {
				if i == 4 {
					continue
				}
				brand := string(payload[i : i+4])
				if brand == "isom" || brand == "mp41" || brand == "mp42" || brand == "avc1" || brand == "dash" || brand == "M4V " || (strings.HasPrefix(brand, "iso") && brand[3] >= '2' && brand[3] <= '9') {
					return "video/mp4"
				}
			}
			break
		}
		if kind != "free" && kind != "skip" {
			break
		}
		offset = end
	}
	if len(data) < 5 || binary.BigEndian.Uint32(data) != 0x1a45dfa3 {
		return ""
	}
	length, n, ok := vint(data[4:], false)
	if !ok || length > uint64(len(data)-4-n) {
		return ""
	}
	end := 4 + n + int(length)
	for pos := 4 + n; pos < end; {
		id, used, valid := vint(data[pos:end], true)
		if !valid {
			return ""
		}
		pos += used
		size, used, valid := vint(data[pos:end], false)
		if !valid {
			return ""
		}
		pos += used
		if size > uint64(end-pos) {
			return ""
		}
		if id == 0x4282 && string(data[pos:pos+int(size)]) == "webm" {
			return "video/webm"
		}
		pos += int(size)
	}
	return ""
}

func vint(data []byte, keepMarker bool) (uint64, int, bool) {
	if len(data) == 0 || data[0] == 0 {
		return 0, 0, false
	}
	n, mask := 1, byte(0x80)
	for data[0]&mask == 0 {
		n++
		mask >>= 1
	}
	if n > 8 || n > len(data) || (keepMarker && n > 4) {
		return 0, 0, false
	}
	value := uint64(data[0])
	if !keepMarker {
		value = uint64(data[0] &^ mask)
	}
	for _, b := range data[1:n] {
		value = value<<8 | uint64(b)
	}
	return value, n, true
}
