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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupRouter(pool *pgxpool.Pool, queries tabmate.Querier) *gin.Engine {
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
		authorized.POST("/api/join-table/:code", tablecontroller.JoinTable(queries)) //join table by code



		authorized.POST("/api/items", tablecontroller.AddMenuItemsToDB(queries))
		authorized.PATCH("/api/items/:id", tablecontroller.UpdateItemQuantity(queries))
		authorized.GET("/api/tables/:code/table-items", tablecontroller.ListItemsWithUserDetailsInTable(queries))
		authorized.DELETE("/api/items/:id", tablecontroller.DeleteItemFromTable(queries))
		authorized.POST("/api/tables/:code/sync", tablecontroller.SyncTableItems(pool))

		authorized.GET("/api/get-user-tables", tablecontroller.ListTablesForUser(queries)) //list tables for user
		authorized.GET("/api/tables/:code/members", tablecontroller.FetchTableMembers(queries)) //fetch table members
		authorized.PATCH("/api/tables/:code/vat", tablecontroller.UpdateTableVat(queries)) //update table VAT

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
	router.GET("/ws/table/:code", func(c *gin.Context) {
		code := c.Param("code")
	token := c.Query("token")

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
		return
	}
	// ✅ Verify OIDC token & get or create user
	user := middleware.VerifyOIDCToken(queries, token)

	// Handle invalid or missing user
	if !user.ID.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing user"})
		return
	}
	// Convert UUID to string
	userIDStr := uuid.UUID(user.ID.Bytes).String()

	log.Printf("WebSocket connection attempt for table code: %s by user: %s (%s)", code, user.Name.String, userIDStr)

	table := tablecontroller.GetTable(code)
	if table == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
		return
	}

	tablecontroller.ServeWsWithUser(
		table,
		c.Writer,
		c.Request,
		user.Name.String,
		user.Email,
	)
	})

	// USERS
	router.POST("/api/create-user", usercontroller.CreateUser(queries))


	// TABLES
	router.GET("/api/tables/:code", tablecontroller.GetTableHandler(queries)) //get table by code
	router.GET("/api/get-user", usercontroller.GetUser(queries))

	// Print all registered routes
	routes := router.Routes()
	log.Println("Registered routes:")
	for _, route := range routes {
		log.Printf("%s %s", route.Method, route.Path)
	}

	return router
}