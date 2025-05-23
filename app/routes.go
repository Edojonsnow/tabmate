package main

import (
	"log"
	"net/http"
	"tabmate/internals/api/controllers"
	"tabmate/internals/api/middleware"
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

	// Protected routes
	authorized := router.Group("/")
	authorized.Use(middleware.AuthMiddleware(queries))
	{
		authorized.GET("/profile", controllers.HandleProfile)
		authorized.GET("/ws-test", func(c *gin.Context) {
			c.HTML(http.StatusOK, "websocket_test.html", nil)
		})
	}
	
	
	// Public routes
	router.GET("/", controllers.HandleHome)
	router.GET("/login", middleware.RedirectIfAuthenticated(), controllers.ShowLoginForm)
	router.POST("/login", controllers.HandleLogin)
	router.GET("/signup", controllers.ShowSignupForm)

	// router.GET("/profile", controllers.ShowProfile)
	router.POST("/signup", controllers.HandleSignup)
	router.GET("/confirm-signup", controllers.HandleConfirmSignup)
	router.POST("/confirm-signup", controllers.HandleConfirmSignup)
	router.GET("/forgot-password", controllers.HandleForgotPassword)
	router.POST("/forgot-password", controllers.HandleForgotPasswordSubmit)
	router.GET("/reset-password", controllers.HandleResetPassword)
	router.POST("/reset-password", controllers.HandleResetPassword)
	router.GET("/callback", controllers.HandleCallback)
	router.GET("/users", controllers.HandleListUsers)
	router.GET("/logout", controllers.HandleLogout)

	router.GET("/ws/table/:code", func(c *gin.Context) {
		code := c.Param("code")
		table := controllers.GetOrCreateTable(code)
		
		// Get the underlying http.ResponseWriter and http.Request
		w := c.Writer
		r := c.Request
		
		// Set the required headers for WebSocket upgrade
		w.Header().Set("Upgrade", "websocket")
		w.Header().Set("Connection", "Upgrade")
		
		controllers.ServeWs(table, w, r)
	})

	// API ROUTES 
	router.POST("/api/create-user", controllers.CreateUser(queries))


	// Print all registered routes
	routes := router.Routes()
	log.Println("Registered routes:")
	for _, route := range routes {
		log.Printf("%s %s", route.Method, route.Path)
	}

	return router
} 