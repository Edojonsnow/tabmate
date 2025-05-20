package controllers

import (
	"context"
	"fmt"
	"net/http"
	tablesclea "tabmate/internals/store/postgres"

	auth "tabmate/internals/auth"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// HandleHome renders the home page with login link
func HandleHome(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{
		"title": "Welcome to Tabmate",
	})
}

// HandleLogin initiates the OAuth2 login flow
func HandleLogin(c *gin.Context) {
	// In production, generate a secure random state
	state := "state" // TODO: Replace with secure random string in production
	
	// Generate the OAuth2 authorization URL using the config from auth package
	url := auth.OAuth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	
	// Redirect to the OAuth2 provider's login page
	c.Redirect(http.StatusFound, url)
}

// HandleCallback processes the OAuth2 callback and gets user information
func HandleCallback(c *gin.Context) {
	// Get the authorization code from the callback
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code is required"})
		return
	}

	// Exchange the code for tokens
	token, err := auth.OAuth2Config.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to exchange code for tokens"})
		return
	}

	// Get user information from the ID token
	userInfo, err := auth.GetUserInfo(context.Background(), token.Extra("id_token").(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get user information"})
		return
	}

	// Store tokens in session/cookies if needed
	// TODO: Implement secure token storage

	// Render the user profile template
	c.HTML(http.StatusOK, "user.html", gin.H{
		"user": userInfo,
	})
}

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

// HandleListUsers displays all users from the Cognito User Pool
func HandleListUsers(c *gin.Context) {
	users, err := auth.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list users: %v", err)})
		return
	}

	c.HTML(http.StatusOK, "users.html", gin.H{
		"users": users,
	})
}