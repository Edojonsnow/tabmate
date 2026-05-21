package splitcontroller

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"tabmate/internals/menu"
	"tabmate/internals/storage"
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

type storedReceiptUpload struct {
	Bytes            []byte
	MediaType        string
	OriginalFilename pgtype.Text
}

func readReceiptUpload(c *gin.Context) (*storedReceiptUpload, error) {
	file, header, err := c.Request.FormFile("receipt_image")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if header.Size > 10*1024*1024 {
		return nil, fmt.Errorf("image too large, max 10MB")
	}

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = "image/jpeg"
	}

	imageBytes, err := io.ReadAll(io.LimitReader(file, 10*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(imageBytes) > 10*1024*1024 {
		return nil, fmt.Errorf("image too large, max 10MB")
	}

	return &storedReceiptUpload{
		Bytes:            imageBytes,
		MediaType:        mediaType,
		OriginalFilename: pgtype.Text{String: header.Filename, Valid: header.Filename != ""},
	}, nil
}

func bindCreateSplitFromReceipt(c *gin.Context, req *CreateSplitFromReceiptRequest) (*storedReceiptUpload, error) {
	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		payload := c.PostForm("payload")
		if payload == "" {
			return nil, fmt.Errorf("payload is required")
		}
		if err := json.Unmarshal([]byte(payload), req); err != nil {
			return nil, err
		}
		upload, err := readReceiptUpload(c)
		if err != nil {
			return nil, err
		}
		return upload, nil
	}
	return nil, c.ShouldBindJSON(req)
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
		receiptUpload, err := bindCreateSplitFromReceipt(c, &req)
		if err != nil {
			log.Printf("Error binding receipt split request: %v", err)
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

		if receiptUpload != nil {
			if _, err := storeSplitReceipt(c, queries, split.ID, splitCode, pgUserID, receiptUpload); err != nil {
				log.Printf("Error storing receipt image: %v", err)
			}
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
			"code":          splitCode,
			"id":            uuid.UUID(split.ID.Bytes).String(),
			"name":          req.Splitname,
			"total_amount":  totalAmount,
			"tax":           req.Tax,
			"tip":           req.Tip,
			"tip_is_shared": req.TipIsShared,
			"split_type":    "receipt",
			"items":         createdItems,
		})
	}
}

func storeSplitReceipt(ctx *gin.Context, queries tabmate.Querier, splitID pgtype.UUID, splitCode string, userID pgtype.UUID, upload *storedReceiptUpload) (tabmate.SplitReceipts, error) {
	r2, err := storage.NewR2Client(ctx)
	if err != nil {
		return tabmate.SplitReceipts{}, err
	}

	ext := "jpg"
	if upload.MediaType == "image/png" {
		ext = "png"
	} else if upload.MediaType == "image/webp" {
		ext = "webp"
	}

	key := fmt.Sprintf("receipts/splits/%s/%s.%s", splitCode, uuid.New().String(), ext)
	object, err := r2.Upload(ctx, key, upload.Bytes, upload.MediaType)
	if err != nil {
		return tabmate.SplitReceipts{}, err
	}

	return queries.UpsertSplitReceipt(ctx, tabmate.UpsertSplitReceiptParams{
		SplitID:          splitID,
		ObjectKey:        object.Key,
		ImageUrl:         object.URL,
		MediaType:        upload.MediaType,
		OriginalFilename: upload.OriginalFilename,
		CreatedBy:        userID,
	})
}

func userIsSplitMember(ctx *gin.Context, queries tabmate.Querier, splitID pgtype.UUID, userID pgtype.UUID) bool {
	_, err := queries.GetSplitMember(ctx, tabmate.GetSplitMemberParams{
		SplitID: splitID,
		UserID:  userID,
	})
	return err == nil
}

// GetSplitReceipt returns the stored original receipt image for a split.
// GET /api/splits/:code/receipt
func GetSplitReceipt(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}
		if !userIsSplitMember(c, queries, split.ID, pgUserID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a split member"})
			return
		}

		receipt, err := queries.GetSplitReceiptBySplitID(c, split.ID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Receipt not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"image_url":         receipt.ImageUrl,
			"object_key":        receipt.ObjectKey,
			"media_type":        receipt.MediaType,
			"original_filename": receipt.OriginalFilename.String,
			"created_at":        receipt.CreatedAt.Time,
			"updated_at":        receipt.UpdatedAt.Time,
		})
	}
}

// UpsertSplitReceipt stores or replaces the original receipt image for a split.
// POST /api/splits/:code/receipt
func UpsertSplitReceipt(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		member, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgUserID,
		})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Not a split member"})
			return
		}
		if member.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the host can update the receipt"})
			return
		}

		upload, err := readReceiptUpload(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		receipt, err := storeSplitReceipt(c, queries, split.ID, code, pgUserID, upload)
		if err != nil {
			log.Printf("Error storing split receipt: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store receipt"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"image_url":         receipt.ImageUrl,
			"object_key":        receipt.ObjectKey,
			"media_type":        receipt.MediaType,
			"original_filename": receipt.OriginalFilename.String,
			"created_at":        receipt.CreatedAt.Time,
			"updated_at":        receipt.UpdatedAt.Time,
		})
	}
}
