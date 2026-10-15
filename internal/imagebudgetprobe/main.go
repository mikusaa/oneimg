// Command imagebudgetprobe measures the real image pipeline in a disposable Linux container.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"mime/multipart"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/utils/images"
)

type pattern struct{ width, height int }

func (p pattern) Bounds() image.Rectangle { return image.Rect(0, 0, p.width, p.height) }
func (p pattern) ColorModel() color.Model { return color.RGBAModel }
func (p pattern) At(x, y int) color.Color {
	return color.RGBA{R: uint8(x / 16), G: uint8(y / 16), B: uint8((x + y) / 32), A: 255}
}

func main() {
	megapixels := flag.Int("mp", 24, "normal image size: 24 or 40 megapixels")
	flag.Parse()
	width, height := 6000, 4000
	if *megapixels == 40 {
		width, height = 8000, 5000
	} else if *megapixels != 24 {
		log.Fatal("only the approved normal 24MP and 40MP probes are supported")
	}
	if runtime.GOOS != "linux" {
		log.Fatal("run this probe in the isolated Linux runtime image, not on the development host")
	}
	file, err := os.CreateTemp("", "oneimg-normal-image-*.jpg")
	must(err)
	defer os.Remove(file.Name())
	defer file.Close()
	// A procedural input avoids allocating another full raster while generating the fixture.
	must(jpeg.Encode(file, pattern{width, height}, &jpeg.Options{Quality: 92}))
	stat, err := file.Stat()
	must(err)
	_, err = file.Seek(0, 0)
	must(err)
	images.InitImageService()
	start := time.Now()
	result, err := images.ImageSvc.ProcessImageContext(context.Background(), file, &multipart.FileHeader{Filename: "normal.jpg", Size: stat.Size()}, models.Settings{
		MaxFileSize: 20 << 20, AllowedTypes: "image/jpeg,image/png,image/gif,image/webp",
		SaveWebp: true, MainImageQuality: 85, Thumbnail: true,
	}, models.RoleAdmin)
	must(err)
	if result.Width != width || result.Height != height || len(result.ThumbnailBytes) == 0 {
		log.Fatal("normal image processing failed")
	}
	status, err := os.ReadFile("/proc/self/status")
	must(err)
	var peakKiB int64
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			peakKiB, err = strconv.ParseInt(strings.Fields(line)[1], 10, 64)
			must(err)
		}
	}
	cgroup, err := os.ReadFile("/sys/fs/cgroup/memory.peak")
	must(err)
	peakBytes, err := strconv.ParseInt(strings.TrimSpace(string(cgroup)), 10, 64)
	must(err)
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{
		"pixels": width * height, "width": width, "height": height, "input_bytes": stat.Size(),
		"output_bytes": len(result.CompressedBytes), "thumbnail_bytes": len(result.ThumbnailBytes),
		"peak_rss_kib": peakKiB, "cgroup_peak_bytes": peakBytes, "elapsed_ms": time.Since(start).Milliseconds(),
		"go_version": runtime.Version(), "arch": runtime.GOARCH,
	}))
	runtime.KeepAlive(result)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
