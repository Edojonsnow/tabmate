package controllers

import (
	"fmt"
	"log"
	"math/big"
	"net/http"
	"tabmate/internals/store/postgres"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateFixedBillRequest struct {
	BillName    string  `json:"billname" binding:"required"`
	Restaurant  string  `json:"restaurant"`
	Description string  `json:"description"`
	TotalAmount float64 `json:"totalAmount" binding:"required"`
}

func CreateFixedBill(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found"})
			return
		}
		
		pgUserID := userID.(pgtype.UUID)
		
		var req CreateFixedBillRequest
	
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		
		billCode := uuid.New().String()[:8]
		
		var totalAmount pgtype.Numeric
		if err := totalAmount.Scan(fmt.Sprintf("%f", req.TotalAmount)); err != nil {
			log.Printf("Error scanning amount: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount"})
			return
		}
		
		// Create bill
		bill, err := queries.CreateFixedBill(c, tabmate.CreateFixedBillParams{
			CreatedBy:      pgUserID,
			BillCode:       billCode,
			Name:           req.BillName,
			Description:    pgtype.Text{String: req.Description, Valid: req.Description != ""},
			TotalAmount:    totalAmount,
		})
		
		if err != nil {
			log.Printf("Error creating bill: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create bill"})
			return
		}
		
		// Add creator as host (they don't owe money)
		_, err = queries.AddUserToBill(c, tabmate.AddUserToBillParams{
			BillID:     bill.ID,
			UserID:     pgUserID,
			AmountOwed: pgtype.Numeric{Int: big.NewInt(0), Valid: true}, // Host doesn't owe TODO: MIGHT NEED TO RENAME THIS FOR BETTER CLARITY, SEEMS CONFUSING
			Role:       "host",
		})
		
		if err != nil {
			log.Printf("Error adding host: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add host"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"code":         billCode,
			"id":           uuid.UUID(bill.ID.Bytes).String(),
			"name":         req.BillName,
			"totalAmount": req.TotalAmount,
		})
	}
}

func JoinFixedBill(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)
		
		// Get bill
		bill, err := queries.GetFixedBillByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
			return
		}
		
		// Check if already a member
		_, err = queries.GetBillMember(c, tabmate.GetBillMemberParams{
			BillID: bill.ID,
			UserID: pgUserID,
		})
		
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Already a member"})
			return
		}
		
		// Calculate current split amount (before adding new member)
		members, _ := queries.ListBillMembersByBillID(c, bill.ID)
		newMemberCount := len(members) + 1
		
		// Total amount as float
		totalAmountFloat, _ := bill.TotalAmount.Float64Value()
		totalAmount := totalAmountFloat.Float64
		amountOwed := totalAmount / float64(newMemberCount)
		
		var amountOwedNumeric pgtype.Numeric
		amountOwedNumeric.Scan(amountOwed)
		
		// Add new member
		_, err = queries.AddUserToBill(c, tabmate.AddUserToBillParams{
			BillID:     bill.ID,
			UserID:     pgUserID,
			AmountOwed: amountOwedNumeric,
			Role:       "guest",
		})
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join bill"})
			return
		}
		
		// Recalculate split for all members
		queries.RecalculateBillSplitForAllMembers(c, bill.ID)
		
		c.JSON(http.StatusOK, gin.H{
			"message":          "Successfully joined bill",
			"amount_owed":      amountOwed,
			"total_amount":     totalAmount,
			"members_count":    newMemberCount,
		})
	}
}

func GetFixedBillByCode(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		
		bill, err := queries.GetFixedBillByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
			return
		}
		
		totalAmountFloat, _ := bill.TotalAmount.Float64Value()
		
		response := gin.H{
			"id":           uuid.UUID(bill.ID.Bytes).String(),
			"code":         bill.BillCode,
			"name":         bill.Name,
			"description":  bill.Description.String,
			"total_amount": totalAmountFloat.Float64,
			"status":       bill.Status,
			"created_at":   bill.CreatedAt.Time,
		}
		
		c.JSON(http.StatusOK, response)
	}
}

func GetBillMembers(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		
		bill, err := queries.GetFixedBillByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
			return
		}
		
		members, err := queries.ListBillMembersWithUserDetails(c, bill.ID)
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

func LeaveBill(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)
		
		bill, err := queries.GetFixedBillByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
			return
		}
		
		err = queries.RemoveUserFromBill(c, tabmate.RemoveUserFromBillParams{
			BillID: bill.ID,
			UserID: pgUserID,
		})
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave bill"})
			return
		}
		
		// Recalculate for remaining members
		queries.RecalculateBillSplitForAllMembers(c, bill.ID)
		
		c.JSON(http.StatusOK, gin.H{"message": "Successfully left the bill"})
	}
}

func GetBillSplit(queries tabmate.Querier) gin.HandlerFunc {
	return GetBillMembers(queries)
}

func MarkAsSettled(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)
		
		bill, err := queries.GetFixedBillByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
			return
		}
		
		_, err = queries.UpdateBillMemberSettledStatus(c, tabmate.UpdateBillMemberSettledStatusParams{
			BillID:    bill.ID,
			UserID:    pgUserID,
			IsSettled: true,
		})
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to settle bill"})
			return
		}
		
		// Check if everyone is settled?
		count, err := queries.CountUnsettledBillMembers(c, bill.ID)
		if err == nil && count == 0 {
			queries.UpdateFixedBillStatus(c, tabmate.UpdateFixedBillStatusParams{
				ID:     bill.ID,
				Status: "settled",
			})
		}
		
		c.JSON(http.StatusOK, gin.H{"message": "Marked as settled"})
	}
}

type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

func AddMemberToBill(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		requesterID, _ := c.Get("user_id")
		pgRequesterID := requesterID.(pgtype.UUID)

		var req AddMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
			return
		}

		// Get bill
		bill, err := queries.GetFixedBillByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Bill not found"})
			return
		}

		// Verify requester is the host
		hostMember, err := queries.GetBillMember(c, tabmate.GetBillMemberParams{
			BillID: bill.ID,
			UserID: pgRequesterID,
		})
		if err != nil || hostMember.Role != "host" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only the bill host can add members"})
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
		_, err = queries.GetUserByID(c, pgTargetID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Check not already a member
		_, err = queries.GetBillMember(c, tabmate.GetBillMemberParams{
			BillID: bill.ID,
			UserID: pgTargetID,
		})
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this bill"})
			return
		}

		// Add member with zero placeholder (recalculate will set the real amount)
		_, err = queries.AddUserToBill(c, tabmate.AddUserToBillParams{
			BillID:     bill.ID,
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
		queries.RecalculateBillSplitForAllMembers(c, bill.ID)

		members, _ := queries.ListBillMembersByBillID(c, bill.ID)
		totalAmountFloat, _ := bill.TotalAmount.Float64Value()

		c.JSON(http.StatusOK, gin.H{
			"message":            "Member added successfully",
			"members_count":      len(members),
			"amount_per_person":  totalAmountFloat.Float64 / float64(len(members)),
		})
	}
}

func ListBillsForUser(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)
		
		bills, err := queries.ListBillsForUser(c, pgUserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bills"})
			return
		}
		
		var response []gin.H
		for _, b := range bills {
			totalAmountFloat, _ := b.TotalAmount.Float64Value()
			amountOwedFloat, _ := b.AmountOwed.Float64Value()
			
			response = append(response, gin.H{
				"id":           uuid.UUID(b.BillID.Bytes).String(),
				"code":         b.BillCode,
				"name":         b.BillName,
				"total_amount": totalAmountFloat.Float64,
				"status":       b.BillStatus,
				"user_role":    b.UserRoleInBill,
				"amount_owed":  amountOwedFloat.Float64,
				"is_settled":   b.UserIsSettled,
				"joined_at":    b.JoinedAt.Time,
			})
		}
		
		c.JSON(http.StatusOK, response)
	}
}