package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"tabmate/internals/auth"
	usercontroller "tabmate/internals/controllers/user"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
    cognitoClient = auth.GetCognitoClient()
    clientID = auth.GetClientID()
	clientSecret = auth.GetClientSecret()
    
    actions = auth.CognitoActions{
        CognitoClient: cognitoClient,
        ClientID:      clientID,
        ClientSecret:  clientSecret,
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

// LoginRequest represents the JSON request from the React frontend
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type SignUpRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required"`
	PhoneNumber string `json:"phone_number" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
}

// LoginResponse represents the JSON response to the React frontend
type LoginResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
	Token       string `json:"token,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	User        *struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user,omitempty"`
}

// HandleLogin processes login requests from React frontend
func HandleLogin(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse JSON request from React frontend
		var loginReq LoginRequest
		if err := c.ShouldBindJSON(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, LoginResponse{
				Success: false,
				Message: "Invalid request format: " + err.Error(),
			})
			return
		}

		if loginReq.Username == "" || loginReq.Password == "" {
			c.JSON(http.StatusBadRequest, LoginResponse{
				Success: false,
				Message: "Username and password are required",
			})
			return
		}

		// Authenticate with Cognito
		authResult, err := actions.SignIn(c.Request.Context(), loginReq.Username, loginReq.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, LoginResponse{
				Success: false,
				Message: "Invalid username or password",
			})
			return
		}

		// Get user info from token
		userInfo, err := auth.GetUserInfo(context.Background(), *authResult.IdToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, LoginResponse{
				Success: false,
				Message: "Failed to get user information",
			})
			return
		}

		// Get user from database and cache it
		user, err := queries.GetUserByEmail(c, userInfo.Email)
		if err != nil {
			fmt.Print("ERR", err)
			// If user does not exist, create them
		    if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows){
				user, err = queries.CreateUser(c, tabmate.CreateUserParams{
					Name:            pgtype.Text{String: userInfo.Name, Valid: true},
					CognitoSub:      userInfo.Sub,
					Email:           userInfo.Email,
				})
				if err != nil {
					log.Printf("Error creating user: %v", err)
					c.Abort()
					return
				}
			} else {
				// Handle other potential errors from GetUserByCognitoSub
				log.Printf("Error checking for existing user: %v", err)
				c.Abort()
				return
			}
		}	
		
		usercontroller.UpdateUserCache(user)

		// Set the token as an HTTP-only cookie for security
		c.SetCookie("auth_token", *authResult.IdToken, 3600, "/", "", false, true)

		// Return success response with token and user info
		c.JSON(http.StatusOK, LoginResponse{
			Success: true,
			Message: "Login successful",
			Token:   *authResult.IdToken,
			AccessToken: *authResult.AccessToken,
			User: &struct {
				ID       string `json:"id"`
				Username string `json:"username"`
				Email    string `json:"email"`
			}{
				ID:       user.ID.String(),
				Username: loginReq.Username,
				Email:    user.Email,
			},
		})
	}
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

	var signupReq SignUpRequest


	if err := c.ShouldBindJSON(&signupReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format: " + err.Error(),
		})
		return
	}

	// Sign up with Cognito
	confirmed , err := actions.SignUp(c.Request.Context(), signupReq.Username, signupReq.Password, signupReq.Email, signupReq.PhoneNumber)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to create account: %v", err),
		})
		return
	}

	if confirmed {
		// If user is automatically confirmed, redirect to login
		c.JSON(http.StatusCreated, gin.H{
			"message": "User registered successfully",
			"username": signupReq.Username,
			"email":    signupReq.Email,
		})
	} else {
		c.Set("username", signupReq.Username)
		c.SetCookie("pending_username", signupReq.Username, 15*60, "/", "", false, true)
		// If user needs to confirm their email, show confirmation page
		c.JSON(http.StatusAccepted, gin.H{
			"message": "Registration initiated. Please check your email to confirm.",
			"username": signupReq.Username,
			"email":    signupReq.Email,
		})
	}
}

// HandleConfirmSignup processes the signup confirmation
func HandleConfirmSignup(c *gin.Context) {
	var confirmReq struct {
		Code     string `json:"code"`
	}

	username, _ := c.Cookie("pending_username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Username not found in cookie",
		})
		return
	}
	    if err := c.ShouldBindJSON(&confirmReq); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid JSON: " + err.Error(),
        })
        return
    }

	    if confirmReq.Code == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Code is required",
        })
        return
    }

	// Confirm the signup with Cognito
	err := actions.ConfirmSignUp(c.Request.Context(), username, confirmReq.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest,  gin.H{
			"error": fmt.Sprintf("Failed to confirm signup: %v", err),
		})
		return
	}

	// Redirect to login after successful confirmation
	c.JSON(http.StatusOK, gin.H{
		"message": "Account confirmed successfully",
	})
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

