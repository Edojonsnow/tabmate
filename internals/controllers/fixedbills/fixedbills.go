package controllers

import (
	"log"
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
	TotalAmount float64 `json:"total_amount" binding:"required,gt=0"`
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		
		billCode := uuid.New().String()[:8]
		
		var totalAmount pgtype.Numeric
		if err := totalAmount.Scan(req.TotalAmount); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount"})
			return
		}
		
		// Create bill
		bill, err := queries.CreateFixedBill(c, tabmate.CreateFixedBillParams{
			CreatedBy:      pgUserID,
			BillCode:       billCode,
			Name:           pgtype.Text{String: req.BillName, Valid: true},
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
			AmountOwed: pgtype.Numeric{Valid: false}, // Host doesn't owe
			Role:       "host",
		})
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add host"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"code":         billCode,
			"id":           uuid.UUID(bill.ID.Bytes).String(),
			"name":         req.BillName,
			"total_amount": req.TotalAmount,
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
		totalAmount := float64(bill.TotalAmount.Int.Int64()) / 100.0
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