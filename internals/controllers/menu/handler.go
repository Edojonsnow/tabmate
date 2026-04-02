package menucontroller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"tabmate/internals/menu"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	scanMu     sync.Mutex
	lastScanAt = make(map[string]time.Time)
)

// ScanMenu accepts a multipart image upload, returns parsed menu items, and persists them to the table.
// POST /api/tables/:code/scan-menu
func ScanMenu(queries tabmate.Querier) gin.HandlerFunc {
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

		tableCode := c.Param("code")

		file, header, err := c.Request.FormFile("menu_image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "menu_image file is required"})
			return
		}
		defer file.Close()

		if header.Size > 5*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "image too large, max 5MB"})
			return
		}

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

		// Persist scanned menu to DB so all members can see it
		menuJSON, err := json.Marshal(items)
		if err != nil {
			log.Printf("Failed to marshal menu items: %v", err)
		} else {
			if err := queries.UpdateTableScannedMenu(c.Request.Context(), tabmate.UpdateTableScannedMenuParams{
				TableCode:   tableCode,
				ScannedMenu: pgtype.Text{String: string(menuJSON), Valid: true},
			}); err != nil {
				log.Printf("Failed to persist scanned menu for table %s: %v", tableCode, err)
			}
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

// GetScannedMenu returns the persisted scanned menu for a table.
// GET /api/tables/:code/menu
func GetScannedMenu(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableCode := c.Param("code")

		scannedMenu, err := queries.GetTableScannedMenu(c.Request.Context(), tableCode)
		if err != nil {
			log.Printf("Failed to get scanned menu for table %s: %v", tableCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve menu"})
			return
		}

		if !scannedMenu.Valid || scannedMenu.String == "" {
			c.JSON(http.StatusOK, gin.H{"items": []any{}})
			return
		}

		var items []menu.MenuItem
		if err := json.Unmarshal([]byte(scannedMenu.String), &items); err != nil {
			log.Printf("Failed to unmarshal scanned menu for table %s: %v", tableCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse stored menu"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

// UpdateScannedMenu replaces the stored menu items for a table.
// PUT /api/tables/:code/menu
func UpdateScannedMenu(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableCode := c.Param("code")

		var body struct {
			Items []menu.MenuItem `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		menuJSON, err := json.Marshal(body.Items)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process menu"})
			return
		}

		if err := queries.UpdateTableScannedMenu(c.Request.Context(), tabmate.UpdateTableScannedMenuParams{
			TableCode:   tableCode,
			ScannedMenu: pgtype.Text{String: string(menuJSON), Valid: true},
		}); err != nil {
			log.Printf("Failed to update scanned menu for table %s: %v", tableCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update menu"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

// DeleteScannedMenu clears the persisted scanned menu for a table.
// DELETE /api/tables/:code/menu
func DeleteScannedMenu(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableCode := c.Param("code")

		if err := queries.UpdateTableScannedMenu(c.Request.Context(), tabmate.UpdateTableScannedMenuParams{
			TableCode:   tableCode,
			ScannedMenu: pgtype.Text{Valid: false},
		}); err != nil {
			log.Printf("Failed to clear scanned menu for table %s: %v", tableCode, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear menu"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
