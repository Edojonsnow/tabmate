package controllers

import (
	"log"
	"net/http"
	"strings"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UserCache stores user details in memory
var userCache = make(map[string]tabmate.Users)

// UpdateUserCache updates the user cache with new user data
func UpdateUserCache(user tabmate.Users) {
	userCache[user.Email] = user
}

// GetUserFromCache retrieves user from cache
func GetUserFromCache(email string) (tabmate.Users, bool) {
	user, exists := userCache[email]
	return user, exists
}

// CreateUser handles user creation
func CreateUser(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
			var req tabmate.CreateUserParams
	
			if err := c.BindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
	
		user, err := queries.CreateUser(c, tabmate.CreateUserParams{
			Name:            req.Name,
			ProfilePictureUrl: req.ProfilePictureUrl,
			CognitoSub:        req.CognitoSub,
			Email:             req.Email,
			})
	
			if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Could not create user."})
			return
			}
	
		c.JSON(http.StatusCreated, user)
	}
}

type UpdatePushTokenRequest struct {
	PushToken string `json:"push_token" binding:"required"`
}

func UpdatePushToken(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		var req UpdatePushTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "push_token is required"})
			return
		}

		err := queries.UpdateUserPushToken(c, tabmate.UpdateUserPushTokenParams{
			PushToken: pgtype.Text{String: req.PushToken, Valid: true},
			ID:        pgUserID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update push token"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func SearchUsers(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		q := strings.TrimSpace(c.Query("q"))
		if len(q) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Search query must be at least 2 characters"})
			return
		}

		users, err := queries.SearchUsersByName(c, tabmate.SearchUsersByNameParams{
			Column1: pgtype.Text{String: q, Valid: true},
			ID:      pgUserID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
			return
		}

		var response []gin.H
		for _, u := range users {
			response = append(response, gin.H{
				"id":                  uuid.UUID(u.ID.Bytes).String(),
				"name":                u.Name.String,
				"email":               u.Email,
				"profile_picture_url": u.ProfilePictureUrl.String,
			})
		}

		if response == nil {
			response = []gin.H{}
		}

		c.JSON(http.StatusOK, response)
	}
}

// GetUser returns the current authenticated user's profile.
// Must be called on a protected route (AuthMiddleware sets user_id in context).
func GetUser(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		user, err := queries.GetUserByID(c, pgUserID)
		if err != nil {
			log.Printf("[GetUser] GetUserByID error: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":             uuid.UUID(user.ID.Bytes).String(),
			"name":           user.Name.String,
			"email":          user.Email,
			"bank_name":      user.BankName.String,
			"account_name":   user.AccountName.String,
			"account_number": user.AccountNumber.String,
		})
	}
}

type UpdateBankDetailsRequest struct {
	BankName      string `json:"bank_name" binding:"required"`
	AccountName   string `json:"account_name" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
}

func UpdateBankDetails(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		pgUserID := userID.(pgtype.UUID)

		var req UpdateBankDetailsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bank_name, account_name and account_number are required"})
			return
		}

		err := queries.UpdateBankDetails(c, tabmate.UpdateBankDetailsParams{
			BankName:      pgtype.Text{String: req.BankName, Valid: true},
			AccountName:   pgtype.Text{String: req.AccountName, Valid: true},
			AccountNumber: pgtype.Text{String: req.AccountNumber, Valid: true},
			ID:            pgUserID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update bank details"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}