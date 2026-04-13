package splitcontroller

import (
	"fmt"
	"log"
	"math/big"
	"net/http"
	"tabmate/internals/notifications"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateSplitRequest struct {
	Splitname   string  `json:"splitname" binding:"required"`
	Description string  `json:"description"`
	TotalAmount float64 `json:"totalAmount" binding:"required"`
}

func CreateSplit(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}

		pgUserID := userID.(pgtype.UUID)

		var req CreateSplitRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		splitCode := uuid.New().String()[:8]

		var totalAmount pgtype.Numeric
		if err := totalAmount.Scan(fmt.Sprintf("%f", req.TotalAmount)); err != nil {
			log.Printf("Error scanning amount: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount"})
			return
		}

		// Create split
		split, err := queries.CreateSplit(c, tabmate.CreateSplitParams{
			CreatedBy:   pgUserID,
			SplitCode:   splitCode,
			Name:        req.Splitname,
			Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
			TotalAmount: totalAmount,
			Status:      "open",
		})

		if err != nil {
			log.Printf("Error creating split: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create split"})
			return
		}

		// Add creator as host (they don't owe money)
		_, err = queries.AddUserToSplit(c, tabmate.AddUserToSplitParams{
			SplitID:    split.ID,
			UserID:     pgUserID,
			AmountOwed: pgtype.Numeric{Int: big.NewInt(0), Valid: true},
			Role:       "host",
		})

		if err != nil {
			log.Printf("Error adding host: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add host"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":        splitCode,
			"id":          uuid.UUID(split.ID.Bytes).String(),
			"name":        req.Splitname,
			"totalAmount": req.TotalAmount,
		})
	}
}

func JoinSplit(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		// Get split
		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		// Check if already a member
		_, err = queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgUserID,
		})

		if err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Already a member"})
			return
		}

		// Calculate current split amount (before adding new member)
		members, _ := queries.ListSplitMembersBySplitID(c, split.ID)
		newMemberCount := len(members) + 1

		// Total amount as float
		totalAmountFloat, _ := split.TotalAmount.Float64Value()
		totalAmount := totalAmountFloat.Float64
		amountOwed := totalAmount / float64(newMemberCount)

		var amountOwedNumeric pgtype.Numeric
		amountOwedNumeric.Scan(amountOwed)

		// Add new member
		_, err = queries.AddUserToSplit(c, tabmate.AddUserToSplitParams{
			SplitID:    split.ID,
			UserID:     pgUserID,
			AmountOwed: amountOwedNumeric,
			Role:       "guest",
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join split"})
			return
		}

		// Recalculate split for all members
		queries.RecalculateSplitForAllMembers(c, split.ID)

		c.JSON(http.StatusOK, gin.H{
			"message":       "Successfully joined split",
			"amount_owed":   amountOwed,
			"total_amount":  totalAmount,
			"members_count": newMemberCount,
		})
	}
}

func GetSplitByCode(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		totalAmountFloat, _ := split.TotalAmount.Float64Value()
		taxAmountFloat, _ := split.TaxAmount.Float64Value()
		tipAmountFloat, _ := split.TipAmount.Float64Value()

		response := gin.H{
			"id":           uuid.UUID(split.ID.Bytes).String(),
			"code":         split.SplitCode,
			"name":         split.Name,
			"description":  split.Description.String,
			"total_amount": totalAmountFloat.Float64,
			"status":       split.Status,
			"split_type":   split.SplitType,
			"tax_amount":   taxAmountFloat.Float64,
			"tip_amount":   tipAmountFloat.Float64,
			"tip_is_shared": split.TipIsShared,
			"created_at":   split.CreatedAt.Time,
		}

		c.JSON(http.StatusOK, response)
	}
}

func GetSplitMembers(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		members, err := queries.ListSplitMembersWithUserDetails(c, split.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
			return
		}

		var response []gin.H
		for _, m := range members {
			amountOwedFloat, _ := m.AmountOwed.Float64Value()

			response = append(response, gin.H{
				"user_id":     uuid.UUID(m.UserID.Bytes).String(),
				"name":        m.UserName.String,
				"email":       m.UserEmail,
				"role":        m.Role,
				"amount_owed": amountOwedFloat.Float64,
				"is_settled":  m.IsSettled,
				"joined_at":   m.JoinedAt.Time,
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

func LeaveSplit(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		err = queries.RemoveUserFromSplit(c, tabmate.RemoveUserFromSplitParams{
			SplitID: split.ID,
			UserID:  pgUserID,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave split"})
			return
		}

		// Recalculate for remaining members
		queries.RecalculateSplitForAllMembers(c, split.ID)

		c.JSON(http.StatusOK, gin.H{"message": "Successfully left the split"})
	}
}

func GetSplitBreakdown(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		members, err := queries.ListSplitMembersWithUserDetails(c, split.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
			return
		}

		// For simple splits just return the equal-split view
		if split.SplitType != "receipt" {
			var response []gin.H
			for _, m := range members {
				amountOwedFloat, _ := m.AmountOwed.Float64Value()
				response = append(response, gin.H{
					"user_id":     uuid.UUID(m.UserID.Bytes).String(),
					"name":        m.UserName.String,
					"email":       m.UserEmail,
					"role":        m.Role,
					"amount_owed": amountOwedFloat.Float64,
					"is_settled":  m.IsSettled,
					"joined_at":   m.JoinedAt.Time,
				})
			}
			c.JSON(http.StatusOK, gin.H{"split_type": "simple", "members": response})
			return
		}

		// Receipt split: build per-member item breakdown
		allClaims, _ := queries.ListClaimsForSplit(c, split.ID)

		taxFloat, _ := split.TaxAmount.Float64Value()
		tipFloat, _ := split.TipAmount.Float64Value()
		memberCount := float64(len(members))

		taxShare := 0.0
		tipShare := 0.0
		if memberCount > 0 {
			taxShare = taxFloat.Float64 / memberCount
			if split.TipIsShared {
				tipShare = tipFloat.Float64 / memberCount
			}
		}

		// Build per-member claimed totals
		claimedByUser := make(map[[16]byte]float64)
		for _, claim := range allClaims {
			priceFloat, _ := claim.ItemPrice.Float64Value()
			claimedByUser[claim.ClaimedByUserID.Bytes] += priceFloat.Float64 * float64(claim.QuantityClaimed)
		}

		var response []gin.H
		for _, m := range members {
			amountOwedFloat, _ := m.AmountOwed.Float64Value()
			claimedItems := claimedByUser[m.UserID.Bytes]

			response = append(response, gin.H{
				"user_id":       uuid.UUID(m.UserID.Bytes).String(),
				"name":          m.UserName.String,
				"email":         m.UserEmail,
				"role":          m.Role,
				"amount_owed":   amountOwedFloat.Float64,
				"claimed_items": claimedItems,
				"tax_share":     taxShare,
				"tip_share":     tipShare,
				"is_settled":    m.IsSettled,
				"joined_at":     m.JoinedAt.Time,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"split_type": "receipt",
			"tax":        taxFloat.Float64,
			"tip":        tipFloat.Float64,
			"tip_is_shared": split.TipIsShared,
			"members":    response,
		})
	}
}

func MarkAsSettled(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		// Block settlement if any items are still unclaimed
		if split.SplitType == "receipt" {
			unclaimedCount, err := queries.CountUnclaimedSplitItems(c, split.ID)
			if err == nil && unclaimedCount > 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":            "All items must be claimed before settling",
					"unclaimed_items":  unclaimedCount,
				})
				return
			}
		}

		_, err = queries.UpdateSplitMemberSettledStatus(c, tabmate.UpdateSplitMemberSettledStatusParams{
			SplitID:   split.ID,
			UserID:    pgUserID,
			IsSettled: true,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to settle split"})
			return
		}

		// Check if everyone is settled
		count, err := queries.CountUnsettledSplitMembers(c, split.ID)
		if err == nil && count == 0 {
			queries.UpdateSplitStatus(c, tabmate.UpdateSplitStatusParams{
				ID:     split.ID,
				Status: "settled",
			})
		}

		c.JSON(http.StatusOK, gin.H{"message": "Marked as settled"})
	}
}

type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func AddMemberToSplit(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		requesterID, _ := c.Get("user_id")
		pgRequesterID := requesterID.(pgtype.UUID)

		var req AddMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
			return
		}

		// Get split
		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		// Verify requester is the host
		hostMember, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgRequesterID,
		})
		if err != nil || hostMember.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the split host can add members"})
			return
		}

		// Parse target user UUID
		targetUUID, err := uuid.Parse(req.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		pgTargetID := pgtype.UUID{Bytes: targetUUID, Valid: true}

		// Check target user exists
		targetUser, err := queries.GetUserByID(c, pgTargetID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Check not already a member
		_, err = queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgTargetID,
		})
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this split"})
			return
		}

		// Add member with zero placeholder (recalculate will set the real amount)
		_, err = queries.AddUserToSplit(c, tabmate.AddUserToSplitParams{
			SplitID:    split.ID,
			UserID:     pgTargetID,
			AmountOwed: pgtype.Numeric{Int: big.NewInt(0), Valid: true},
			Role:       "guest",
		})
		if err != nil {
			log.Printf("Error adding member: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
			return
		}

		// Recalculate split for everyone
		queries.RecalculateSplitForAllMembers(c, split.ID)

		members, _ := queries.ListSplitMembersBySplitID(c, split.ID)
		totalAmountFloat, _ := split.TotalAmount.Float64Value()

		// Send push notification to the added user (fire-and-forget)
		if targetUser.PushToken.Valid && targetUser.PushToken.String != "" {
			hostUser, err := queries.GetUserByID(c, pgRequesterID)
			hostName := "Someone"
			if err == nil && hostUser.Name.Valid {
				hostName = hostUser.Name.String
			}

			go func() {
				err := notifications.SendExpoPushNotification(notifications.ExpoMessage{
					To:    targetUser.PushToken.String,
					Title: "You've been added to a split",
					Body:  fmt.Sprintf("%s added you to \"%s\"", hostName, split.Name),
					Data:  map[string]string{"splitCode": split.SplitCode},
				})
				if err != nil {
					log.Printf("Failed to send push notification to user %s: %v", req.UserID, err)
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"message":           "Member added successfully",
			"members_count":     len(members),
			"amount_per_person": totalAmountFloat.Float64 / float64(len(members)),
		})
	}
}

func RemoveMemberFromSplit(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		requesterID, _ := c.Get("user_id")
		pgRequesterID := requesterID.(pgtype.UUID)

		// Parse target user ID from URL param
		targetUUID, err := uuid.Parse(c.Param("userId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		pgTargetID := pgtype.UUID{Bytes: targetUUID, Valid: true}

		// Get split
		split, err := queries.GetSplitByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Split not found"})
			return
		}

		// Verify requester is the host
		hostMember, err := queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgRequesterID,
		})
		if err != nil || hostMember.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the split host can remove members"})
			return
		}

		// Prevent host from removing themselves
		if pgRequesterID == pgTargetID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Host cannot remove themselves from the split"})
			return
		}

		// Check target is actually a member
		_, err = queries.GetSplitMember(c, tabmate.GetSplitMemberParams{
			SplitID: split.ID,
			UserID:  pgTargetID,
		})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User is not a member of this split"})
			return
		}

		// Remove the member
		err = queries.RemoveUserFromSplit(c, tabmate.RemoveUserFromSplitParams{
			SplitID: split.ID,
			UserID:  pgTargetID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
			return
		}

		// Recalculate split for remaining members
		queries.RecalculateSplitForAllMembers(c, split.ID)

		members, _ := queries.ListSplitMembersBySplitID(c, split.ID)
		totalAmountFloat, _ := split.TotalAmount.Float64Value()

		c.JSON(http.StatusOK, gin.H{
			"message":           "Member removed successfully",
			"members_count":     len(members),
			"amount_per_person": totalAmountFloat.Float64 / float64(len(members)),
		})
	}
}

func ListSplitsForUser(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		splits, err := queries.ListSplitsForUser(c, pgUserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch splits"})
			return
		}

		var response []gin.H
		for _, s := range splits {
			totalAmountFloat, _ := s.TotalAmount.Float64Value()
			amountOwedFloat, _ := s.AmountOwed.Float64Value()

			var settledAt any
			if s.SettledAt.Valid {
				settledAt = s.SettledAt.Time
			}

			response = append(response, gin.H{
				"id":           uuid.UUID(s.SplitID.Bytes).String(),
				"code":         s.SplitCode,
				"name":         s.SplitName,
				"total_amount": totalAmountFloat.Float64,
				"status":       s.SplitStatus,
				"user_role":    s.UserRoleInSplit,
				"amount_owed":  amountOwedFloat.Float64,
				"is_settled":   s.UserIsSettled,
				"joined_at":    s.JoinedAt.Time,
				"settled_at":   settledAt,
			})
		}

		c.JSON(http.StatusOK, response)
	}
}
