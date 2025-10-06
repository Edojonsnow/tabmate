package middleware

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"

	"tabmate/internals/auth"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// AuthMiddleware checks if the user is authenticated
func AuthMiddleware(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the cookie
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
            c.Abort()
            return
        }
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Verify the token using OIDC provider
		userInfo, err := auth.GetUserInfo(context.Background(), tokenString)
		if err != nil {
			log.Printf("Invalid auth token: %v, redirecting to login", err)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Check if user already exists
		user, err := queries.GetUserByCognitoSub(c, userInfo.Sub)
		if err != nil {
			// If user does not exist, create them
			if err == sql.ErrNoRows {
				user, err = queries.CreateUser(c, tabmate.CreateUserParams{
					Name:            pgtype.Text{String: userInfo.Name, Valid: true},
					CognitoSub:      userInfo.Sub,
					Email:           userInfo.Email,
				})
				if err != nil {
					log.Printf("Error creating user: %v", err)
					c.Redirect(http.StatusFound, "/login")
					c.Abort()
					return
				}
			} else {
				// Handle other potential errors from GetUserByCognitoSub
				log.Printf("Error checking for existing user: %v", err)
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
		}			

		// Set user info in context
		c.Set("username", userInfo.Name)
		c.Set("email", userInfo.Email)
		c.Set("user_id", user.ID)
		c.Next()
	}
}

// RedirectIfAuthenticated redirects to /profile if user is already logged in
func RedirectIfAuthenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("auth_token")
		if err == nil {
			// Token exists, verify it
			_, err := auth.GetUserInfo(context.Background(), token)
			if err == nil {
				// Valid token, redirect to profile
				c.Redirect(http.StatusFound, "/profile")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}