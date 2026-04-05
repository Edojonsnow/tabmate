package splitcontroller

import (
	"fmt"
	"log"
	"net/http"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetSplitItems returns all items for a split with claim details.
// GET /api/splits/:code/items
func GetSplitItems(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		items, err := queries.ListSplitItems(c, split.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
			return
		}

		// Fetch all claims for this split in one query
		allClaims, err := queries.ListClaimsForSplit(c, split.ID)
		if err != nil {
			log.Printf("Failed to fetch claims for split %s: %v", code, err)
			allClaims = []tabmate.ListClaimsForSplitRow{}
		}

		// Index claims by item id
		claimsByItem := make(map[[16]byte][]gin.H)
		for _, claim := range allClaims {
			claimsByItem[claim.SplitItemID.Bytes] = append(claimsByItem[claim.SplitItemID.Bytes], gin.H{
				"user_id":          uuid.UUID(claim.ClaimedByUserID.Bytes).String(),
				"user_name":        claim.UserName.String,
				"quantity_claimed": claim.QuantityClaimed,
			})
		}

		var response []gin.H
		for _, item := range items {
			priceFloat, _ := item.Price.Float64Value()
			claims := claimsByItem[item.ID.Bytes]
			if claims == nil {
				claims = []gin.H{}
			}
			response = append(response, gin.H{
				"id":            uuid.UUID(item.ID.Bytes).String(),
				"name":          item.Name,
				"price":         priceFloat.Float64,
				"quantity":      item.Quantity,
				"remaining_qty": item.RemainingQty,
				"claims":        claims,
			})
		}

		if response == nil {
			response = []gin.H{}
		}

		c.JSON(http.StatusOK, response)
	}
}

// ClaimItem lets a member claim N units of an item.
// POST /api/splits/:code/items/:itemId/claim
func ClaimItem(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		itemIDStr := c.Param("itemId")

		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		var body struct {
			Quantity int `json:"quantity" binding:"required,min=1"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "quantity is required and must be >= 1"})
			return
		}

		// Verify split exists and user is a member
		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		if _, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgUserID,
		}); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not a member of this split"})
			return
		}

		// Get the item
		itemUUID, err := uuid.Parse(itemIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
			return
		}
		pgItemID := pgtype.UUID{Bytes: itemUUID, Valid: true}

		item, err := queries.GetSplitItem(c, pgItemID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
			return
		}

		// Check if user already has a claim on this item
		existingClaim, existingErr := queries.GetSplitItemClaim(c, tabmate.GetSplitItemClaimParams{
			SplitItemID:      pgItemID,
			ClaimedByUserID:  pgUserID,
		})

		// Calculate how much of remaining_qty this new claim uses
		// If re-claiming, we return the previous quantity first
		previousQty := int32(0)
		if existingErr == nil {
			previousQty = existingClaim.QuantityClaimed
		}

		availableQty := item.RemainingQty + previousQty // restore previous claim
		if int32(body.Quantity) > availableQty {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         fmt.Sprintf("Only %d unit(s) available to claim", availableQty),
				"available_qty": availableQty,
			})
			return
		}

		// Upsert the claim
		if _, err := queries.AddSplitItemClaim(c, tabmate.AddSplitItemClaimParams{
			SplitItemID:     pgItemID,
			ClaimedByUserID: pgUserID,
			QuantityClaimed: int32(body.Quantity),
		}); err != nil {
			log.Printf("Error upserting claim: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to claim item"})
			return
		}

		// Update remaining_qty: start from total quantity, subtract all claims
		newRemaining := availableQty - int32(body.Quantity)
		if _, err := queries.UpdateSplitItemRemainingQty(c, tabmate.UpdateSplitItemRemainingQtyParams{
			ID:           pgItemID,
			RemainingQty: newRemaining,
		}); err != nil {
			log.Printf("Error updating remaining_qty: %v", err)
		}

		// Recalculate this member's amount_owed
		recalculateMemberAmount(c, queries, split, pgUserID)

		c.JSON(http.StatusOK, gin.H{
			"message":       "Item claimed",
			"remaining_qty": newRemaining,
		})
	}
}

// UnclaimItem removes a member's claim on an item.
// DELETE /api/splits/:code/items/:itemId/claim
func UnclaimItem(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		itemIDStr := c.Param("itemId")

		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		itemUUID, err := uuid.Parse(itemIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
			return
		}
		pgItemID := pgtype.UUID{Bytes: itemUUID, Valid: true}

		// Get existing claim to restore quantity
		claim, err := queries.GetSplitItemClaim(c, tabmate.GetSplitItemClaimParams{
			SplitItemID:     pgItemID,
			ClaimedByUserID: pgUserID,
		})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No claim found for this item"})
			return
		}

		// Delete the claim
		if err := queries.DeleteSplitItemClaim(c, tabmate.DeleteSplitItemClaimParams{
			SplitItemID:     pgItemID,
			ClaimedByUserID: pgUserID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove claim"})
			return
		}

		// Restore remaining_qty
		item, err := queries.GetSplitItem(c, pgItemID)
		if err == nil {
			newRemaining := item.RemainingQty + claim.QuantityClaimed
			queries.UpdateSplitItemRemainingQty(c, tabmate.UpdateSplitItemRemainingQtyParams{
				ID:           pgItemID,
				RemainingQty: newRemaining,
			})
		}

		// Recalculate this member's amount_owed
		recalculateMemberAmount(c, queries, split, pgUserID)

		c.JSON(http.StatusOK, gin.H{"message": "Claim removed"})
	}
}

type receiptItemInput struct {
	Name     string  `json:"name" binding:"required"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity" binding:"required,min=1"`
}

// ingestItems is the shared logic for adding a slice of items to a split and updating its total.
// Used by both ReplaceAllSplitItems and MergeSplitItems.
func ingestItems(c *gin.Context, queries tabmate.Querier, split tabmate.Splits, pgUserID pgtype.UUID, newItems []receiptItemInput) ([]gin.H, error) {
	var created []gin.H
	for _, item := range newItems {
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
			log.Printf("Error adding item %s: %v", item.Name, err)
			continue
		}
		priceFloat, _ := si.Price.Float64Value()
		created = append(created, gin.H{
			"id":            uuid.UUID(si.ID.Bytes).String(),
			"name":          si.Name,
			"price":         priceFloat.Float64,
			"quantity":      si.Quantity,
			"remaining_qty": si.RemainingQty,
			"claims":        []gin.H{},
		})
	}

	// Recalculate total = sum of all items + tax + (tip if shared)
	allItems, err := queries.ListSplitItems(c, split.ID)
	if err == nil {
		var itemsTotal float64
		for _, i := range allItems {
			p, _ := i.Price.Float64Value()
			itemsTotal += p.Float64 * float64(i.Quantity)
		}
		taxFloat, _ := split.TaxAmount.Float64Value()
		tipFloat, _ := split.TipAmount.Float64Value()
		newTotal := itemsTotal + taxFloat.Float64
		if split.TipIsShared {
			newTotal += tipFloat.Float64
		}
		var totalNumeric pgtype.Numeric
		totalNumeric.Scan(fmt.Sprintf("%.2f", newTotal))
		queries.UpdateSplitTotalAmount(c, tabmate.UpdateSplitTotalAmountParams{
			ID:          split.ID,
			TotalAmount: totalNumeric,
		})
	}

	return created, nil
}

type scanItemsRequest struct {
	Items []receiptItemInput `json:"items" binding:"required,min=1"`
}

// ReplaceAllSplitItems wipes all existing items (and their claims) and inserts a new set.
// Host only. PUT /api/splits/:code/items
func ReplaceAllSplitItems(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		member, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{SplitID: split.ID, UserID: pgUserID})
		if err != nil || member.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the host can update receipt items"})
			return
		}

		var req scanItemsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Wipe all existing items (cascade deletes claims too)
		if err := queries.DeleteAllSplitItems(c, split.ID); err != nil {
			log.Printf("Error deleting split items: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear existing items"})
			return
		}

		created, _ := ingestItems(c, queries, split, pgUserID, req.Items)
		if created == nil {
			created = []gin.H{}
		}
		c.JSON(http.StatusOK, gin.H{"items": created})
	}
}

// MergeSplitItems appends new items to the existing item list without touching claims.
// Host only. POST /api/splits/:code/items
func MergeSplitItems(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		member, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{SplitID: split.ID, UserID: pgUserID})
		if err != nil || member.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the host can add receipt items"})
			return
		}

		var req scanItemsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		created, _ := ingestItems(c, queries, split, pgUserID, req.Items)
		if created == nil {
			created = []gin.H{}
		}
		c.JSON(http.StatusOK, gin.H{"items": created})
	}
}

// recalculateMemberAmount recomputes a member's amount_owed for a receipt-based split.
// amount = sum(claimed items) + (tax / member_count) + (tip / member_count if shared)
func recalculateMemberAmount(c *gin.Context, queries tabmate.Querier, split tabmate.Splits, userID pgtype.UUID) {
	if split.SplitType != "receipt" {
		return
	}

	allClaims, err := queries.ListClaimsForSplit(c, split.ID)
	if err != nil {
		return
	}

	members, err := queries.ListSplitMembersBySplitID(c, split.ID)
	if err != nil {
		return
	}
	memberCount := float64(len(members))
	if memberCount == 0 {
		return
	}

	taxFloat, _ := split.TaxAmount.Float64Value()
	tipFloat, _ := split.TipAmount.Float64Value()

	taxShare := taxFloat.Float64 / memberCount
	tipShare := 0.0
	if split.TipIsShared {
		tipShare = tipFloat.Float64 / memberCount
	}

	// Sum this user's claimed items
	var claimedTotal float64
	for _, claim := range allClaims {
		if claim.ClaimedByUserID == userID {
			priceFloat, _ := claim.ItemPrice.Float64Value()
			claimedTotal += priceFloat.Float64 * float64(claim.QuantityClaimed)
		}
	}

	newAmount := claimedTotal + taxShare + tipShare

	var amountNumeric pgtype.Numeric
	amountNumeric.Scan(fmt.Sprintf("%.2f", newAmount))

	queries.UpdateSplitMemberAmount(c, tabmate.UpdateSplitMemberAmountParams{
		SplitID:    split.ID,
		UserID:     userID,
		AmountOwed: amountNumeric,
	})
}
