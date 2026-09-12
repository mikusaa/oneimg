package videos

import (
	"os"
	"testing"
)

func TestDetectContainers(t *testing.T) {
	for _, format := range []string{"mp4", "webm"} {
		data, err := os.ReadFile("testdata/sample." + format)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) > HeaderLimit {
			data = data[:HeaderLimit]
		}
		if got := Detect(data); got != "video/"+format {
			t.Fatalf("%s detected as %q", format, got)
		}
		for _, size := range []int{0, 1, 4, 5, 8, 12} {
			if Detect(data[:size]) != "" {
				t.Fatalf("accepted truncated %s (%d)", format, size)
			}
		}
	}
	for _, data := range [][]byte{[]byte("not a video"), []byte("fake ftyp isom"), {0x1a, 0x45, 0xdf, 0xa3, 0x87, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'p'}} {
		if got := Detect(data); got != "" {
			t.Fatalf("accepted fake %x: %s", data, got)
		}
	}
}
