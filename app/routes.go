package main

import (
	"log"
	"tabmate/internals/api/controllers"
	tablesclea "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
)

func setupRouter(queries tablesclea.Querier) *gin.Engine {
	router := gin.Default()
	
	// Add logging middleware
	router.Use(gin.Logger())
	
	// Debug: Print all registered routes
	router.Use(func(c *gin.Context) {
		log.Printf("Request: %s %s", c.Request.Method, c.Request.URL.Path)
	})
	
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.POST("/api/signup", controllers.CreateUser(queries))
	
	// Print all registered routes
	routes := router.Routes()
	log.Println("Registered routes:")
	for _, route := range routes {
		log.Printf("%s %s", route.Method, route.Path)
	}

	return router
} 