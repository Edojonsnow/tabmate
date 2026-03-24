// Package controllers handles HTTP requests. Auth is now managed by Clerk on the frontend.
// This file retains only web-facing handlers needed for HTML templates.
package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleHome(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{"title": "Welcome to Tabmate"})
}

func HandleProfile(c *gin.Context) {
	username, _ := c.Get("username")
	email, _ := c.Get("email")
	c.HTML(http.StatusOK, "profile.html", gin.H{
		"title":    "Profile",
		"username": username,
		"email":    email,
	})
}
