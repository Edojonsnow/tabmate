package controllers

import (
	"net/http"
	tablesclea "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
)

// CreateUser handles user creation
func CreateUser(queries tablesclea.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
			var req tablesclea.CreateUserParams
	
			if err := c.BindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
	
		user, err := queries.CreateUser(c, tablesclea.CreateUserParams{
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