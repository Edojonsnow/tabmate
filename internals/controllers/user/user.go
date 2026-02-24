package controllers

import (
	"context"
	"net/http"
	"strings"
	"tabmate/internals/auth"
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

func GetUser(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the cookie
		token, err := c.Cookie("auth_token")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		userInfo, err := auth.GetUserInfo(context.Background(), token)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		
		// Try to get user from cache first
		if user, exists := GetUserFromCache(userInfo.Email); exists {
			c.JSON(http.StatusOK, user)
			return
		}
		// If not in cache, get from database
		user, err := queries.GetUserByEmail(c, userInfo.Email)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Could not find user in Database"})
			return
		}
		
		// Update cache with new user data
		UpdateUserCache(user)
		c.JSON(http.StatusOK, user)
	}
}