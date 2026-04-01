package menucontroller

import (
	"fmt"
	"log"
	"net/http"
	"tabmate/internals/menu"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	scanMu   sync.Mutex
	lastScanAt = make(map[string]time.Time)
)

// ScanMenu accepts a multipart image upload and returns parsed menu items.
// POST /api/tables/:code/scan-menu
func ScanMenu() gin.HandlerFunc {
	return func(c *gin.Context) {

		userID := c.MustGet("user_id")
		uid := fmt.Sprintf("%v", userID)

		scanMu.Lock()
		lastTime, ok := lastScanAt[uid]
		if ok && time.Since(lastTime) < 30*time.Second {
			scanMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Please wait 30 seconds before scanning again"})
			return
		}
		lastScanAt[uid] = time.Now()
		scanMu.Unlock()

		file, header, err := c.Request.FormFile("menu_image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "menu_image file is required"})
			return
		}
		defer file.Close()

		// Validate size: max 5MB
		if header.Size > 5*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "image too large, max 5MB"})
			return
		}

		// Detect media type from Content-Type header or default to jpeg
		mediaType := header.Header.Get("Content-Type")
		if mediaType == "" {
			mediaType = "image/jpeg"
		}

		imageBytes := make([]byte, header.Size)
		if _, err := file.Read(imageBytes); err != nil {
			log.Printf("Error reading uploaded image: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image"})
			return
		}

		items, err := menu.ScanMenuImage(imageBytes, mediaType)
		if err != nil {
			log.Printf("Menu scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse menu"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}
