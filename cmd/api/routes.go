package main

import (
	"log"
	"net/http"
	authcontroller "tabmate/internals/controllers/auth"
	tablecontroller "tabmate/internals/controllers/table"
	usercontroller "tabmate/internals/controllers/user"
	"tabmate/internals/middleware"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
)

func setupRouter(queries tabmate.Querier) *gin.Engine {
	router := gin.Default()
	
	// Load HTML templates
	router.LoadHTMLGlob("templates/*")
	
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
		authorized.GET("/profile", authcontroller.HandleProfile)
		
		authorized.GET("/ws-test", func(c *gin.Context) {
			username, _ := c.Get("username")
			email, _ := c.Get("email")
			userID, _ := c.Get("user_id") // Retrieve user_id from context

			c.HTML(http.StatusOK, "websocket_test.html", gin.H{
				"username": username,
				"email":    email,
				"userID":   userID, // Pass user_id to the template
			})
		})

		
		authorized.POST("/api/create-table", tablecontroller.CreateTable(queries))
		authorized.POST("/api/tables/add-item-to-order", tablecontroller.AddItemToTable(queries))
		authorized.GET("/api/tables/:code/table-items", tablecontroller.ListItemsWithUserDetailsInTable(queries))

	}
	
	
	// Public routes
	router.GET("/", authcontroller.HandleHome)
	router.GET("/login", middleware.RedirectIfAuthenticated(), authcontroller.ShowLoginForm)
	router.POST("/login", authcontroller.HandleLogin(queries))
	router.GET("/signup", authcontroller.ShowSignupForm)
	router.POST("/signup", authcontroller.HandleSignup)
	router.GET("/confirm-signup", authcontroller.HandleConfirmSignup)
	router.POST("/confirm-signup", authcontroller.HandleConfirmSignup)
	router.GET("/forgot-password", authcontroller.HandleForgotPassword)
	router.POST("/forgot-password", authcontroller.HandleForgotPasswordSubmit)
	router.GET("/reset-password", authcontroller.HandleResetPassword)
	router.POST("/reset-password", authcontroller.HandleResetPassword)
	router.GET("/callback", authcontroller.HandleCallback)
	router.GET("/users", authcontroller.HandleListUsers)
	router.GET("/logout", authcontroller.HandleLogout)

	// WebSocket route with authentication
	authorized.GET("/ws/table/:code", func(c *gin.Context) {
		code := c.Param("code")
		username, _ := c.Get("username")
		email, _ := c.Get("email")
		
		log.Printf("WebSocket connection attempt for table code: %s by user: %s", code, username)
		
		table := tablecontroller.GetTable(code)
		if table == nil {
			log.Printf("Table not found in activeTables map for code: %s", code)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Table not found",
			})
			return
		}
		
		log.Printf("Found table in activeTables map, establishing WebSocket connection for user: %s", username)
		tablecontroller.ServeWsWithUser(table, c.Writer, c.Request, username.(string), email.(string))
	})

	// API ROUTES 

	// USERS
	router.POST("/api/create-user", usercontroller.CreateUser(queries))


	// TABLES

	router.GET("/api/tables/:code", tablecontroller.GetTableHandler(queries)) //check if table exists
	router.GET("/api/get-user", usercontroller.GetUser(queries))

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