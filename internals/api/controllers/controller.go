package controllers

import (
	"context"
	"fmt"
	"net/http"
	cognito "tabmate/cognito"
	"tabmate/internals/auth"

	"github.com/gin-gonic/gin"
)

var (
	cognitoClient = auth.GetCognitoClient()
	clientID = auth.GetClientID()
	
	actions = cognito.CognitoActions{
		CognitoClient: cognitoClient,
	}
)

// HandleHome renders the home page with login link
func HandleHome(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{
		"title": "Welcome to Tabmate",
	})
}

// ShowLoginForm renders the login form
func ShowLoginForm(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
	})
}

// HandleLogin processes the login form submission
func HandleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" || password == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "Username and password are required",
		})
		return
	}

	// Authenticate with Cognito
	authResult, err := actions.SignIn(c.Request.Context(), username, password)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	// Store the ID token in a cookie
	c.SetCookie("auth_token", *authResult.IdToken, 3600, "/", "", false, true)

	// Redirect to profile page
	c.Redirect(http.StatusFound, "/profile")
}

// ShowSignupForm renders the signup form
func ShowSignupForm(c *gin.Context) {
	c.HTML(http.StatusOK, "signup.html", gin.H{
		"title": "Sign Up",
	})
}

// func ShowProfile(c *gin.Context) {
// 	c.HTML(http.StatusOK, "profile.html", gin.H{
// 		"title": "Profile",
// 	})
// }

// HandleProfile displays the user's profile
func HandleProfile(c *gin.Context) {
	username, _ := c.Get("username")
	email, _ := c.Get("email")

	c.HTML(http.StatusOK, "profile.html", gin.H{
		"title":    "Profile",
		"username": username,
		"email":    email,
	})
}

// HandleSignup processes the signup form submission
func HandleSignup(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	email := c.PostForm("email")
	name := c.PostForm("name")
	phone := c.PostForm("phone")

	if username == "" || password == "" || email == "" || name == "" || phone == "" {
		c.HTML(http.StatusBadRequest, "signup.html", gin.H{
			"error": "All fields are required",
		})
		return
	}

	// Sign up with Cognito
	confirmed , err := actions.SignUp(c.Request.Context(), username, password, email, phone)
	if err != nil {
		c.HTML(http.StatusBadRequest, "signup.html", gin.H{
			"error": fmt.Sprintf("Failed to create account: %v", err),
		})
		return
	}
	if confirmed {
		// If user is automatically confirmed, redirect to login
		c.Redirect(http.StatusFound, "/login")
	} else {
		// If user needs to confirm their email, show confirmation page
		c.HTML(http.StatusOK, "confirm_signup.html", gin.H{
			"username": username,
			"email":    email,
		})
	}
}

// HandleConfirmSignup processes the signup confirmation
func HandleConfirmSignup(c *gin.Context) {
	username := c.PostForm("username")
	// email := c.PostForm("email")
	code := c.PostForm("code")

	if username == "" || code == "" {
		c.HTML(http.StatusBadRequest, "confirm_signup.html", gin.H{
			"error": "Username and confirmation code are required",
		})
		return
	}

	// Confirm the signup with Cognito
	err := actions.ConfirmSignUp(c.Request.Context(), username, code)
	if err != nil {
		c.HTML(http.StatusBadRequest, "confirm_signup.html", gin.H{
			"error": fmt.Sprintf("Failed to confirm signup: %v", err),
		})
		return
	}



	// Redirect to login after successful confirmation
	c.Redirect(http.StatusFound, "/login")
}

// HandleForgotPassword shows the forgot password form
func HandleForgotPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot_password.html", gin.H{
		"title": "Forgot Password",
	})
}

// HandleForgotPasswordSubmit processes the forgot password request
func HandleForgotPasswordSubmit(c *gin.Context) {
	username := c.PostForm("username")

	if username == "" {
		c.HTML(http.StatusBadRequest, "forgot_password.html", gin.H{
			"error": "Username is required",
		})
		return
	}

	// Initiate password reset
	_, err := actions.ForgotPassword(c.Request.Context(), username)
	if err != nil {
		c.HTML(http.StatusBadRequest, "forgot_password.html", gin.H{
			"error": fmt.Sprintf("Failed to initiate password reset: %v", err),
		})
		return
	}

	// Show confirmation page
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"username": username,
	})
}

// HandleResetPassword processes the password reset
func HandleResetPassword(c *gin.Context) {
	username := c.PostForm("username")
	code := c.PostForm("code")
	newPassword := c.PostForm("new_password")

	if username == "" || code == "" || newPassword == "" {
		c.HTML(http.StatusBadRequest, "reset_password.html", gin.H{
			"error": "All fields are required",
		})
		return
	}

	// Confirm password reset
	err := actions.ConfirmForgotPassword(c.Request.Context(), code, username, newPassword)
	if err != nil {
		c.HTML(http.StatusBadRequest, "reset_password.html", gin.H{
			"error": fmt.Sprintf("Failed to reset password: %v", err),
		})
		return
	}

	// Redirect to login
	c.Redirect(http.StatusFound, "/login")
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

// HandleLogout handles user logout
func HandleLogout(c *gin.Context) {
	// Clear the auth token cookie
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
	
	// Redirect to home page
	c.Redirect(http.StatusFound, "/")
}

