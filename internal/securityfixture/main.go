// Command securityfixture serves disposable data on loopback for browser regression tests.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"oneimg/backend/app"
	"oneimg/backend/config"
	"oneimg/backend/database"
	"oneimg/backend/models"
	"oneimg/backend/routes"
	"oneimg/backend/services"
)

func main() {
	root, err := os.Getwd()
	must(err)
	temp, err := os.MkdirTemp("", "oneimg-security-browser-*")
	must(err)
	defer os.RemoveAll(temp)
	must(os.Chdir(temp))
	must(os.Mkdir("uploads", 0700))
	cfg := &config.Config{AppURL: "http://127.0.0.1:18089", SqlitePath: filepath.Join(temp, "test.db"), SessionSecret: "test-only-session-secret", ConfigSecret: "test-only-config-secret"}
	config.App = cfg
	database.InitDB(cfg)
	db := database.GetDB()
	hash, err := bcrypt.GenerateFromPassword([]byte("browser-test-password"), bcrypt.MinCost)
	must(err)
	must(db.DB.Create(&models.User{ID: 1, Username: "browser-test", Password: string(hash), Role: models.RoleAdmin}).Error)
	must(db.DB.Create(&models.Settings{}).Error)
	must(db.DB.Create(&models.Buckets{Id: 1, Name: `storage"><script>window.previewInjected=1</script>`, Type: "default"}).Error)
	tag := models.Tags{Name: `tag"><img src=x onerror="window.previewInjected=1">`}
	must(db.DB.Create(&tag).Error)
	filename := `legacy"#? onerror="window.previewInjected=1.png`
	file, err := os.Create(filepath.Join("uploads", filename))
	must(err)
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 70, B: 35, A: 255})
		}
	}
	must(png.Encode(file, img))
	must(file.Close())
	record := models.Image{Id: 1, Url: "/uploads/" + filename, FileName: filename, OriginalFileName: `photo"><img src=x onerror="window.previewInjected=1">.png`, MimeType: "image/png", Width: 32, Height: 24, Storage: "default", BucketId: 1, UserId: 1, FileSize: 128}
	must(db.DB.Create(&record).Error)
	must(db.DB.Create(&models.ImageToTags{ImageId: 1, TagId: tag.Id}).Error)
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><style>@font-face{font-family:evil;src:url('/svg-font')}text{font-family:evil}</style><rect width="80" height="80" fill="red"/><script>window.svgInjected=1;fetch('/svg-leak')</script><image href="/svg-external"/><text y="20">text</text><foreignObject width="100" height="100"><div xmlns="http://www.w3.org/1999/xhtml"><iframe src="/svg-frame"></iframe><form action="/svg-form"><button type="submit">submit</button></form></div></foreignObject></svg>`
	must(os.WriteFile("uploads/legacy.svg", []byte(svg), 0600))
	must(db.DB.Create(&models.Image{Id: 2, Url: "/uploads/legacy.svg", FileName: "legacy.svg", MimeType: "image/svg+xml", Width: 100, Height: 100, Storage: "default", BucketId: 1, UserId: 1}).Error)
	system := &app.System{Config: cfg, Database: db, Services: services.New(db, cfg)}
	router := routes.SetupRoutes(os.DirFS(root), system)
	var attempts atomic.Int64
	for _, path := range []string{"/svg-control", "/svg-leak", "/svg-external", "/svg-font", "/svg-frame", "/svg-form"} {
		router.Any(path, func(c *gin.Context) {
			attempts.Add(1)
			c.Status(http.StatusNoContent)
		})
	}
	router.GET("/__security/csp-requests", func(c *gin.Context) { c.JSON(http.StatusOK, attempts.Load()) })
	log.Println("Disposable browser fixture ready on loopback :18089")
	must(http.ListenAndServe("127.0.0.1:18089", router))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
