package controllers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	tabmate "tabmate/internals/store/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Table struct {
	ID      uuid.UUID
	Code    string
	clients map[*TableClient]bool
	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *TableClient

	// Unregister requests from clients.
	unregister chan *TableClient

	processIncomingMessage chan *ClientMessage
}

type ClientMessage struct {
	Client *TableClient
	Data   []byte
}

type CreateTableReq struct {
	TableName  string `json:"tablename" binding:"required"`
	Restaurant string `json:"restaurant" binding:"required"`
}

type ItemDelta struct {
	ItemName      string      `json:"itemName"`
	Price         float64     `json:"price"`
	QuantityDelta int         `json:"quantityDelta"`
	Username      string      `json:"username"`
	AddedByUserID pgtype.UUID `json:"addedByUserId"`
}

type BulkSyncRequest struct {
	Updates []ItemDelta `json:"updates"`
}

// Map to track active tables
var activeTables = make(map[string]*Table)

func InitializeActiveTables(ctx context.Context, db *tabmate.Queries) error {
	//  TODO: Modify this function to initialize tables from db based on status  'open'
	codes, err := db.GetAllTableCodes(ctx)
	if err != nil {
		return err
	}

	for _, code := range codes {
		if _, exists := activeTables[code]; !exists {
			// Get table details from database
			dbTable, err := db.GetTableByCode(ctx, code)
			if err != nil {
				log.Printf("Error getting table details for code %s: %v", code, err)
				continue
			}
			// Create new table instance
			table := NewTable(dbTable.ID.Bytes, code)
			activeTables[code] = table
			// Start the table's goroutine
			go table.Run()
		}
	}
	return nil
}

func GetTable(code string) *Table {
	log.Printf("Looking for table %s in activeTables map. Current active tables: %v", code, activeTables)
	if table, exists := activeTables[code]; exists {
		return table
	}
	return nil
}

func GetTables(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tables, err := queries.ListTablesByStatus(c, "open")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tables"})
			return
		}
		c.JSON(http.StatusOK, tables)
	}
}

func CreateTable(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve user_id from context
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
			return
		}

		var createTableReq struct {
			TableName  string `json:"tablename" binding:"required"`
			Restaurant string `json:"restaurant" binding:"required"`
		}

		if err := c.ShouldBindJSON(&createTableReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Type assert userID to pgtype.UUID
		pgUserID, ok := userID.(pgtype.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assert user ID type"})
			return
		}
		// Generate a new table code
		newTableCode := uuid.New().String()[:8]

		// Create table in database
		dbTable, err := queries.CreateTable(c, tabmate.CreateTableParams{
			CreatedBy:      pgUserID,
			TableCode:      newTableCode,
			Name:           pgtype.Text{String: createTableReq.TableName, Valid: true},
			RestaurantName: pgtype.Text{String: createTableReq.Restaurant, Valid: true},
			Status:         "open",
			MenuUrl:        pgtype.Text{Valid: false},
			Members:        []int32{int32(pgUserID.Bytes[15])},
		})
		if err != nil {
			log.Printf("Database error creating table: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create table due to a database error. Please try again later."})
			return
		}

		_, err = queries.AddUserToTable(c, tabmate.AddUserToTableParams{
			TableID: dbTable.ID,
			UserID:  pgUserID,
			Role:    "host",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to table due to a database error. Please try again later."})
			return
		}

		// Create new table instance
		newTable := NewTable(dbTable.ID.Bytes, newTableCode)
		activeTables[newTableCode] = newTable

		// Start the table's goroutine
		go newTable.Run()

		c.JSON(http.StatusOK, gin.H{
			"code":       newTableCode,
			"id":         uuid.UUID(dbTable.ID.Bytes).String(),
			"name":       createTableReq.TableName,
			"restaurant": createTableReq.Restaurant,
			"created_by": pgUserID,
		})
	}
}

func JoinTable(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		userId, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
			return
		}
		// Type assert userId to pgtype.UUID
		pgUserID := userId.(pgtype.UUID)

		// Fetch Table from Database
		dbTable, err := queries.GetTableByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Table not found in database"})
			return
		}

		// Check if user is already a member of the table
		userExists, err := userIsMember(c, queries, dbTable.ID, pgUserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if !userExists {
			_, err := queries.AddUserToTable(c, tabmate.AddUserToTableParams{
				TableID: dbTable.ID,
				UserID:  pgUserID,
				Role:    "guest",
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"code": code,
			"id":   uuid.UUID(dbTable.ID.Bytes).String(),
			// "usernames": usernames,
			"tablename":  dbTable.Name,
			"restaurant": dbTable.RestaurantName,
			"host":       uuid.UUID(dbTable.CreatedBy.Bytes).String(),
		})

	}
}

func FetchTableMembers(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		dbTable, err := queries.GetTableByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Table not found in database"})
			return
		}

		tableMembers, err := queries.ListMembersWithUserDetailsByTableID(c, dbTable.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch table members"})
		}

		c.JSON(http.StatusOK, tableMembers)
	}
}

func userIsMember(ctx context.Context, queries tabmate.Querier, tableID, userID pgtype.UUID) (bool, error) {
	_, err := queries.GetTableMember(ctx, tabmate.GetTableMemberParams{
		TableID: tableID,
		UserID:  userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // user not found, but no problem
		}
		return false, err // actual DB error
	}
	return true, nil
}

func AddItemToTable(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve user_id from context
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
			return
		}

		// Type assert userID to pgtype.UUID
		pgUserID, ok := userID.(pgtype.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assert user ID type"})
			return
		}

		var req tabmate.AddItemToTableParams
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		// Set default values for fields not provided by the frontend
		if req.Quantity == 0 {
			req.Quantity = 1 // Default quantity to 1
		}
		if !req.Description.Valid {
			req.Description = pgtype.Text{String: "", Valid: false} // Default empty description
		}
		if !req.OriginalParsedText.Valid {
			req.OriginalParsedText = pgtype.Text{String: req.Name, Valid: true} // Default to item name
		}

		// Set the AddedByUserID from the context
		req.AddedByUserID = pgUserID

		newItem, err := queries.AddItemToTable(c, req)
		if err != nil {
			log.Printf("Database error adding item to table: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add item to table due to a database error. Please try again later."})
			return
		}

		c.JSON(http.StatusOK, newItem)
	}
}

func UpdateItemQuantity(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {

		id := c.Param("id")
		var req struct {
			Quantity int32 `json:"quantity"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		updatedItem, err := queries.UpdateItemQuantity(c, tabmate.UpdateItemQuantityParams{
			ID:       pgtype.UUID{Bytes: uuid.MustParse(id), Valid: true},
			Quantity: req.Quantity,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item quantity"})
			return
		}

		c.JSON(http.StatusOK, updatedItem)
	}
}

func AddMenuItemsToDB(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {

		var req []tabmate.AddItemToTableParams
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		fmt.Println("Payload", req)

		for _, item := range req {
			// Set default values for fields not provided by the frontend
			if item.Quantity == 0 {
				item.Quantity = 1 // Default quantity to 1
			}
			if !item.Description.Valid {
				item.Description = pgtype.Text{String: "", Valid: false} // Default empty description
			}
			if !item.OriginalParsedText.Valid {
				item.OriginalParsedText = pgtype.Text{String: item.Name, Valid: true} // Default to item name
			}

			_, err := queries.AddItemToTable(c, item)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add item to table due to a database error. Please try again later."})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Items added to table successfully"})
	}
}

func SyncTableItems(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		tableCode := c.Param("code")
		if tableCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Table ID missing"})
			return
		}

		var req BulkSyncRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin database transaction"})
			return
		}
		defer tx.Rollback(ctx) // safe rollback if something fails

		q := tabmate.New(tx) // sqlc Queries USING the transaction

		existingItems, err := q.ListItemsWithUserDetailsInTable(ctx, tableCode)
		if err != nil {
			tx.Rollback(ctx)
			c.JSON(500, gin.H{"error": "failed to list items"})
			return
		}

		// Create lookup map
		itemsMap := make(map[string]tabmate.ListItemsWithUserDetailsInTableRow)
		for _, it := range existingItems {
			key := strings.ToLower(it.Name) + ":" + it.AddedByUserID.String()
			itemsMap[key] = it
		}

		for _, upd := range req.Updates {
			key := strings.ToLower(upd.ItemName) + ":" + upd.AddedByUserID.String()
			existing, found := itemsMap[key]

			var price pgtype.Numeric

			if err := price.Scan(fmt.Sprintf("%.2f", upd.Price)); err != nil {
				log.Printf("price scan error: %v, raw=%v", err, upd.Price)
				c.JSON(500, gin.H{"error": "Invalid price value"})
				return
			}

			if !found {
				// New item, only add if delta > 0
				if upd.QuantityDelta > 0 {
					_, err := q.AddItemToTable(ctx, tabmate.AddItemToTableParams{
						TableCode:          tableCode,
						AddedByUserID:      upd.AddedByUserID,
						Name:               upd.ItemName,
						Price:              price,
						Quantity:           int32(upd.QuantityDelta),
						Description:        pgtype.Text{},
						OriginalParsedText: pgtype.Text{},
					})
					if err != nil {
						c.JSON(500, gin.H{"error": err.Error()})
						return
					}
				}
				continue
			}

			newQty := existing.Quantity + int32(upd.QuantityDelta)

			if newQty <= 0 {
				// delete
				if err := q.DeleteItemFromTable(ctx, existing.ID); err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
			} else {
				// update
				_, err := q.UpdateItemQuantity(ctx, tabmate.UpdateItemQuantityParams{
					ID:       existing.ID,
					Quantity: newQty,
				})
				if err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
			}
		}

		if err := tx.Commit(ctx); err != nil {
			c.JSON(500, gin.H{"error": "Failed to commit transaction"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})

	}
}

func DeleteItemFromTable(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {

		// Convert string to pgtype.UUID
		var itemID struct {
			Id pgtype.UUID `json:"id"`
		}

		if err := c.ShouldBindJSON(&itemID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
			return
		}

		err := queries.DeleteItemFromTable(c, itemID.Id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
			return
		}
	}
}

func ListItemsInTable(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		items, err := queries.ListItemsInTable(c, code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list items in table"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func ListItemsWithUserDetailsInTable(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		items, err := queries.ListItemsWithUserDetailsInTable(c, code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list items with user details in table"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func ListTablesForUser(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve user_id from context
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
			return
		}

		// Type assert userID to pgtype.UUID
		pgUserID, ok := userID.(pgtype.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assert user ID type"})
			return
		}

		tables, err := queries.ListTablesWithMembershipStatusForUser(c, pgUserID)
		if err != nil {
			log.Printf("ListTablesWithMembershipStatusForUser error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tables with membership status for user"})
			return
		}

		c.JSON(http.StatusOK, tables)
	}
}

func GetTableHandler(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")

		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Table code is required"})
			return
		}
		// Check if table exists in database
		dbTable, err := queries.GetTableByCode(c, code)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
			return
		}

		// Get or create table instance
		table := GetTable(code)
		if table == nil {
			table = NewTable(dbTable.ID.Bytes, code)
			activeTables[code] = table
			go table.Run()
		}
		// Get connected usernames
		usernames := table.GetUsernames()
		log.Printf("Connected usernames for table %s: %v", code, usernames)

		c.JSON(http.StatusOK, gin.H{
			"code":       code,
			"id":         dbTable.ID,
			"usernames":  usernames,
			"tablename":  dbTable.Name,
			"restaurant": dbTable.RestaurantName,
		})
	}
}

func (t *Table) GetUsernames() []string {
	usernames := []string{}
	for client := range t.clients {
		log.Printf("Client in map: %v", client.username)
		if client.username != "" {
			usernames = append(usernames, client.username)
		}
	}
	return usernames
}

func NewTable(id uuid.UUID, code string) *Table {
	return &Table{
		ID:                     id,
		Code:                   code,
		clients:                make(map[*TableClient]bool),
		broadcast:              make(chan []byte),
		register:               make(chan *TableClient),
		unregister:             make(chan *TableClient),
		processIncomingMessage: make(chan *ClientMessage),
	}
}

func UpdateTableVat(queries tabmate.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tableCode := c.Param("code")

		var req struct {
			Vat pgtype.Numeric `json:"vat"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		updatedTable, err := queries.UpdateTableVat(c, tabmate.UpdateTableVatParams{
			TableCode:  tableCode,
			Vat: req.Vat,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update table VAT"})
			return
		}

		c.JSON(http.StatusOK, updatedTable)
	}
}

func (t *Table) Run() {
	for {
		select {
		case client := <-t.register:
			log.Printf("Registering client: %v", client.username)
			t.clients[client] = true
			usernames := t.GetUsernames()
			log.Printf("Current connected usernames: %v", usernames)

		case client := <-t.unregister:
			if _, ok := t.clients[client]; ok {
				log.Printf("Client unregistered: %v", client.username)
				delete(t.clients, client)
				close(client.send)

			}

		case clientMsg := <-t.processIncomingMessage:
			// 1. Process clientMsg.Data (e.g., parse JSON, identify action like "addItem")
			// 2. Call relevant service (e.g., itemService.AddItem(...))
			// 3. If successful, prepare a message to broadcast (e.g., "newItemAdded")
			// 4. Send this message to t.broadcast
			// Example:
			// newItemMsg := []byte(`{"type": "itemAdded", "payload": ...}`)
			// t.broadcast <- newItemMsg
			log.Printf("Table %s received message from client %s: %s", t.Code, clientMsg.Client.userID, string(clientMsg.Data))

		case messageToBroadcast := <-t.broadcast:
			log.Printf("Broadcasting message to %d clients in table %s", len(t.clients), t.Code)
			for client := range t.clients {
				select {
				case client.send <- messageToBroadcast:
					log.Printf("Message sent to client in table %s", t.Code)
				default:
					log.Printf("Failed to send message to client in table %s", t.Code)
					close(client.send)
					delete(t.clients, client)
				}
			}
		}
	}
}
