package middleware

import (
	"context"
	"net/http"
	"tabmate/internals/auth"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware checks if the user is authenticated
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the cookie
		token, err := c.Cookie("auth_token")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Verify the token using OIDC provider
		userInfo, err := auth.GetUserInfo(context.Background(), token)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("username", userInfo.Name)
		c.Set("email", userInfo.Email)
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