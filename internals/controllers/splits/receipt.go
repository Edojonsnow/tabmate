package splitcontroller

import (
	"fmt"
	"log"
	"math/big"
	"net/http"
	"tabmate/internals/menu"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// PreviewReceipt scans a receipt image and returns extracted items/tax/tip without any DB writes.
// POST /api/splits/preview-receipt
func PreviewReceipt() gin.HandlerFunc {
	return func(c *gin.Context) {
		file, header, err := c.Request.FormFile("receipt_image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "receipt_image file is required"})
			return
		}
		defer file.Close()

		if header.Size > 10*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "image too large, max 10MB"})
			return
		}

		mediaType := header.Header.Get("Content-Type")
		if mediaType == "" {
			mediaType = "image/jpeg"
		}

		imageBytes := make([]byte, header.Size)
		if _, err := file.Read(imageBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read image"})
			return
		}

		receipt, err := menu.ScanReceiptImage(imageBytes, mediaType)
		if err != nil {
			log.Printf("Receipt scan error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse receipt"})
			return
		}

		c.JSON(http.StatusOK, receipt)
	}
}

// CreateSplitFromReceiptRequest is the body for creating a receipt-based split.
type CreateSplitFromReceiptRequest struct {
	Splitname   string  `json:"splitname" binding:"required"`
	Description string  `json:"description"`
	TipIsShared bool    `json:"tip_is_shared"`
	Tax         float64 `json:"tax"`
	Tip         float64 `json:"tip"`
	Items       []struct {
		Name     string  `json:"name" binding:"required"`
		Price    float64 `json:"price"`
		Quantity int     `json:"quantity" binding:"required,min=1"`
	} `json:"items" binding:"required,min=1"`
}

// CreateSplitFromReceipt creates a split with pre-scanned receipt items in a single call.
// POST /api/create-split-from-receipt
func CreateSplitFromReceipt(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		pgUserID := userID.(pgtype.UUID)

		var req CreateSplitFromReceiptRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Calculate total = sum(item price * qty) + tax + (tip if shared)
		var itemsTotal float64
		for _, item := range req.Items {
			itemsTotal += item.Price * float64(item.Quantity)
		}
		totalAmount := itemsTotal + req.Tax
		if req.TipIsShared {
			totalAmount += req.Tip
		}

		splitCode := uuid.New().String()[:8]

		var totalAmountNumeric pgtype.Numeric
		if err := totalAmountNumeric.Scan(fmt.Sprintf("%.2f", totalAmount)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount"})
			return
		}

		// Create the split
		split, err := queries.CreateSplit(c, tabmate.CreateSplitParams{
			CreatedBy:   pgUserID,
			SplitCode:   splitCode,
			Name:        req.Splitname,
			Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
			TotalAmount: totalAmountNumeric,
			Status:      "open",
		})
		if err != nil {
			log.Printf("Error creating split: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create split"})
			return
		}

		// Set tax/tip/split_type on the split
		var taxNumeric, tipNumeric pgtype.Numeric
		taxNumeric.Scan(fmt.Sprintf("%.2f", req.Tax))
		tipNumeric.Scan(fmt.Sprintf("%.2f", req.Tip))

		if _, err := queries.UpdateSplitReceiptDetails(c, tabmate.UpdateSplitReceiptDetailsParams{
			ID:          split.ID,
			TaxAmount:   taxNumeric,
			TipAmount:   tipNumeric,
			TipIsShared: req.TipIsShared,
			TotalAmount: totalAmountNumeric,
		}); err != nil {
			log.Printf("Error updating receipt details: %v", err)
			// Non-fatal — split is still created
		}

		// Add creator as host
		if _, err := queries.AddUserToSplit(c, tabmate.AddUserToSplitParams{
			SplitID:    split.ID,
			UserID:     pgUserID,
			AmountOwed: pgtype.Numeric{Int: big.NewInt(0), Valid: true},
			Role:       "host",
		}); err != nil {
			log.Printf("Error adding host: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add host"})
			return
		}

		// Insert all receipt items
		type createdItem struct {
			ID           string  `json:"id"`
			Name         string  `json:"name"`
			Price        float64 `json:"price"`
			Quantity     int     `json:"quantity"`
			RemainingQty int     `json:"remaining_qty"`
		}
		var createdItems []createdItem

		for _, item := range req.Items {
			var priceNumeric pgtype.Numeric
			priceNumeric.Scan(fmt.Sprintf("%.2f", item.Price))

			si, err := queries.AddSplitItem(c, tabmate.AddSplitItemParams{
				SplitID:       split.ID,
				Name:          item.Name,
				Price:         priceNumeric,
				Quantity:      int32(item.Quantity),
				AddedByUserID: pgUserID,
			})
			if err != nil {
				log.Printf("Error adding split item %s: %v", item.Name, err)
				continue
			}

			priceFloat, _ := si.Price.Float64Value()
			createdItems = append(createdItems, createdItem{
				ID:           uuid.UUID(si.ID.Bytes).String(),
				Name:         si.Name,
				Price:        priceFloat.Float64,
				Quantity:     int(si.Quantity),
				RemainingQty: int(si.RemainingQty),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"code":         splitCode,
			"id":           uuid.UUID(split.ID.Bytes).String(),
			"name":         req.Splitname,
			"total_amount": totalAmount,
			"tax":          req.Tax,
			"tip":          req.Tip,
			"tip_is_shared": req.TipIsShared,
			"split_type":   "receipt",
			"items":        createdItems,
		})
	}
}
