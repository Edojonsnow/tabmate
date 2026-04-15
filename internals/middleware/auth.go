package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"tabmate/internals/auth"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// AuthMiddleware verifies a Clerk session token and resolves/auto-creates the DB user.
func AuthMiddleware(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			c.Abort()
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		userInfo, err := auth.VerifyClerkToken(tokenString)
		if err != nil {
			log.Printf("Invalid Clerk token: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Look up user by Clerk user ID (stored in cognito_sub column).
		user, err := queries.GetUserByCognitoSub(c, userInfo.Sub)
		if err != nil {
			// Auto-create on first authenticated request.
			user, err = queries.CreateUser(c, tabmate.CreateUserParams{
				Name:       pgtype.Text{String: userInfo.Name, Valid: userInfo.Name != ""},
				CognitoSub: userInfo.Sub,
				Email:      userInfo.Email,
			})
			if err != nil {
				log.Printf("[AuthMiddleware] CreateUser error for sub=%q email=%q: %v", userInfo.Sub, userInfo.Email, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
				c.Abort()
				return
			}
			log.Printf("[AuthMiddleware] auto-created user id=%v for sub=%q", user.ID, userInfo.Sub)
		}

		c.Set("username", userInfo.Name)
		c.Set("email", userInfo.Email)
		c.Set("user_id", user.ID)
		c.Next()
	}
}

// VerifyOIDCToken is used by the WebSocket route to authenticate without middleware.
func VerifyOIDCToken(queries tabmate.Querier, tokenString string) tabmate.Users {
	userInfo, err := auth.VerifyClerkToken(tokenString)
	if err != nil {
		log.Printf("Invalid Clerk token (WS): %v", err)
		return tabmate.Users{}
	}

	ctx := context.Background()
	user, err := queries.GetUserByCognitoSub(ctx, userInfo.Sub)
	if err != nil {
		user, err = queries.CreateUser(ctx, tabmate.CreateUserParams{
			Name:       pgtype.Text{String: userInfo.Name, Valid: userInfo.Name != ""},
			CognitoSub: userInfo.Sub,
			Email:      userInfo.Email,
		})
		if err != nil {
			log.Printf("Error auto-creating user (WS): %v", err)
			return tabmate.Users{}
		}
	}

	return user
}
