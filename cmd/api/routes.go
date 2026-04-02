package main

import (
	"log"
	"net/http"
	authcontroller "tabmate/internals/controllers/auth"
	tablecontroller "tabmate/internals/controllers/table"
	usercontroller "tabmate/internals/controllers/user"
	billcontroller "tabmate/internals/controllers/fixedbills"
	menucontroller "tabmate/internals/controllers/menu"
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
	router.Use(func(c *gin.Context) {
		log.Printf("Request: %s %s", c.Request.Method, c.Request.URL.Path)
	})

	// ─── Public routes ────────────────────────────────────────────────────────
	router.GET("/", authcontroller.HandleHome)

	// ─── Protected routes ─────────────────────────────────────────────────────
	authorized := router.Group("/")
	authorized.Use(middleware.AuthMiddleware(queries))
	{
		authorized.GET("/profile", authcontroller.HandleProfile)

		authorized.GET("/ws-test", func(c *gin.Context) {
			username, _ := c.Get("username")
			email, _ := c.Get("email")
			userID, _ := c.Get("user_id")
			c.HTML(http.StatusOK, "websocket_test.html", gin.H{
				"username": username,
				"email":    email,
				"userID":   userID,
			})
		})

		// ── User ──────────────────────────────────────────────────────────────
		authorized.GET("/api/me", usercontroller.GetUser(queries))
		authorized.GET("/api/users/search", usercontroller.SearchUsers(queries))
		authorized.PATCH("/api/user/push-token", usercontroller.UpdatePushToken(queries))

		// ── Tables ────────────────────────────────────────────────────────────
		authorized.POST("/api/create-table", tablecontroller.CreateTable(queries))
		authorized.POST("/api/tables/add-item-to-order", tablecontroller.AddItemToTable(queries))
		authorized.POST("/api/join-table/:code", tablecontroller.JoinTable(queries))
		authorized.GET("/api/tables/:code", tablecontroller.GetTableHandler(queries))
		authorized.GET("/api/tables/:code/members", tablecontroller.FetchTableMembers(queries))
		authorized.GET("/api/tables/:code/table-items", tablecontroller.ListItemsWithUserDetailsInTable(queries))
		authorized.GET("/api/get-user-tables", tablecontroller.ListTablesForUser(queries))

		// Table Items
		authorized.POST("/api/items", tablecontroller.AddMenuItemsToDB(queries))
		authorized.PATCH("/api/items/:id", tablecontroller.UpdateItemQuantity(queries))
		authorized.DELETE("/api/items/:id", tablecontroller.DeleteItemFromTable(queries))
		authorized.POST("/api/tables/:code/sync", tablecontroller.SyncTableItems(pool))
		authorized.PATCH("/api/tables/:code", tablecontroller.UpdateTableVat(queries))
		authorized.POST("/api/tables/:code/scan-menu", menucontroller.ScanMenu(queries))
		authorized.GET("/api/tables/:code/menu", menucontroller.GetScannedMenu(queries))

		// ── Fixed Bills ───────────────────────────────────────────────────────
		authorized.POST("/api/create-bill", billcontroller.CreateFixedBill(queries))
		authorized.GET("/api/bills/:code", billcontroller.GetFixedBillByCode(queries))
		authorized.POST("/api/join-bill/:code", billcontroller.JoinFixedBill(queries))
		authorized.POST("/api/bills/:code/add-member", billcontroller.AddMemberToBill(queries))
		authorized.GET("/api/bills/:code/members", billcontroller.GetBillMembers(queries))
		authorized.DELETE("/api/bills/:code/leave", billcontroller.LeaveBill(queries))
		authorized.DELETE("/api/bills/:code/members/:userId", billcontroller.RemoveMemberFromBill(queries))
		authorized.GET("/api/bills/:code/split", billcontroller.GetBillSplit(queries))
		authorized.POST("/api/bills/:code/settle", billcontroller.MarkAsSettled(queries))
		authorized.GET("/api/get-user-bills", billcontroller.ListBillsForUser(queries))
	}

	// ─── WebSocket (token via query param) ────────────────────────────────────
	router.GET("/ws/table/:code", func(c *gin.Context) {
		code := c.Param("code")
		token := c.Query("token")

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing token"})
			return
		}

		user := middleware.VerifyOIDCToken(queries, token)
		if !user.ID.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing user"})
			return
		}

		userIDStr := uuid.UUID(user.ID.Bytes).String()
		log.Printf("WebSocket connection for table %s by user %s (%s)", code, user.Name.String, userIDStr)

		table := tablecontroller.GetTable(code)
		if table == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
			return
		}

		tablecontroller.ServeWsWithUser(table, c.Writer, c.Request, user.Name.String, user.Email)
	})

	// Print registered routes
	routes := router.Routes()
	log.Println("Registered routes:")
	for _, route := range routes {
		log.Printf("%s %s", route.Method, route.Path)
	}

	return router
}
