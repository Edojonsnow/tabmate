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
			username, _ := c.Get("username")
			email, _ := c.Get("email")
		
			c.HTML(http.StatusOK, "websocket_test.html", gin.H{
				"username": username,
				"email":    email,
			})
		})

		
	}
	
	
	// Public routes
	router.GET("/", controllers.HandleHome)
	router.GET("/login", middleware.RedirectIfAuthenticated(), controllers.ShowLoginForm)
	router.POST("/login", controllers.HandleLogin(queries))
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

	// WebSocket route with authentication
	authorized.GET("/ws/table/:code", func(c *gin.Context) {
		code := c.Param("code")
		username, _ := c.Get("username")
		email, _ := c.Get("email")
		
		log.Printf("WebSocket connection attempt for table code: %s by user: %s", code, username)
		
		table := controllers.GetTable(code)
		if table == nil {
			log.Printf("Table not found in activeTables map for code: %s", code)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Table not found",
			})
			return
		}
		
		log.Printf("Found table in activeTables map, establishing WebSocket connection for user: %s", username)
		controllers.ServeWsWithUser(table, c.Writer, c.Request, username.(string), email.(string))
	})

	// API ROUTES 

	// USERS
	router.POST("/api/create-user", controllers.CreateUser(queries))


	// TABLES
	router.POST("/api/create-table", controllers.CreateTable(queries))
	router.GET("/api/tables/:code", controllers.GetTableHandler(queries)) //check if table exists
	router.GET("/api/get-user", controllers.GetUser(queries))

	// Table routes
	// router.GET("/tables", controllers.GetTables(queries))
	// router.POST("/tables", controllers.CreateTable(queries))
	// router.GET("/tables/:code", controllers.GetTableHandler(queries))

	// Print all registered routes
	routes := router.Routes()
	log.Println("Registered routes:")
	for _, route := range routes {
		log.Printf("%s %s", route.Method, route.Path)
	}

	return router
} 