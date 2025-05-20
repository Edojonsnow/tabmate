package main

import (
	"log"
	"tabmate/internals/api/controllers"
	tablesclea "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
)

func setupRouter(queries tablesclea.Querier) *gin.Engine {
	router := gin.Default()
	
	// Load HTML templates
	router.LoadHTMLGlob("app/templates/*")
	
	// Add logging middleware
	router.Use(gin.Logger())
	
	// Debug: Print all registered routes
	router.Use(func(c *gin.Context) {
		log.Printf("Request: %s %s", c.Request.Method, c.Request.URL.Path)
	})
	
	// Public routes
	router.GET("/", controllers.HandleHome)
	router.GET("/login", controllers.HandleLogin)
	// router.GET("/callback", controllers.HandleCallback)
	// router.GET("/logout", controllers.HandleLogout)
	router.POST("/api/signup", controllers.CreateUser(queries))

	// Print all registered routes
	routes := router.Routes()
	log.Println("Registered routes:")
	for _, route := range routes {
		log.Printf("%s %s", route.Method, route.Path)
	}

	return router
} 