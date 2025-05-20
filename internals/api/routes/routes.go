package routes

import (
	"tabmate/internals/api/controllers"
	tablesclea "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all the routes for the application
func SetupRoutes(router *gin.Engine, queries tablesclea.Querier) {
	// Load HTML templates
	router.LoadHTMLGlob("templates/*")

	// Public routes
	router.GET("/", controllers.HandleHome)
	router.GET("/login", controllers.HandleLogin)
	router.GET("/callback", controllers.HandleCallback)
	router.GET("/users", controllers.HandleListUsers)

	// API routes
	api := router.Group("/api")
	{
		api.POST("/users", controllers.CreateUser(queries))
	}
} 