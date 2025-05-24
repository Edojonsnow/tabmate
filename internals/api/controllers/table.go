package controllers

import (
	"context"
	"log"
	"tabmate/internals/auth"
	tablesclea "tabmate/internals/store/postgres"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Table struct {
	ID uuid.UUID
	Code string
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
	Data []byte

}

// Map to track active tables
var activeTables = make(map[string]*Table)

func InitializeActiveTables(ctx context.Context, db *tablesclea.Queries) error {
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

func GetTables(queries tablesclea.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		tables, err := queries.ListTablesByStatus(c, "open")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tables"})
			return
		}
		c.JSON(http.StatusOK, tables)
	}
}

func CreateTable(queries tablesclea.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user info from token
		token, err := c.Cookie("auth_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}

		userInfo, err := auth.GetUserInfo(c, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}
		// Get User from memory cache
		user, exists := GetUserFromCache(userInfo.Email)
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		// Generate a new table code
		newTableCode := uuid.New().String()[:8]

		// Create table in database
		dbTable, err := queries.CreateTable(c, tablesclea.CreateTableParams{
			CreatedBy:      user.ID,
			TableCode:      newTableCode,
			Name:           pgtype.Text{String: "New Table", Valid: true},
			RestaurantName: pgtype.Text{String: "Restaurant", Valid: true},
			Status:         "open",
			MenuUrl:        pgtype.Text{Valid: false},
			Members:        []int32{int32(uuid.MustParse(userInfo.Sub).ID())},
		})
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create table"})
			return
		}

		// Create new table instance
		newTable := NewTable(dbTable.ID.Bytes, newTableCode)
		activeTables[newTableCode] = newTable

		// Start the table's goroutine
		go newTable.Run()

		c.JSON(http.StatusOK, gin.H{
			"code": newTableCode,
			"id":   dbTable.ID,
		})
	}
}

func GetTableHandler(queries tablesclea.Querier) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Param("code")
		
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

		c.JSON(http.StatusOK, gin.H{
			"code": code,
			"id":   dbTable.ID,
		})
	}
}

func NewTable(id uuid.UUID , code string ) *Table{
	return &Table{
		ID:          id,
        Code:        code,
        clients:     make(map[*TableClient]bool),
        broadcast:   make(chan []byte), 
        register:    make(chan *TableClient),
        unregister:  make(chan *TableClient),
        processIncomingMessage: make(chan *ClientMessage),
	}
}

func (t *Table) Run(){
	for {
        select {
        case client := <-t.register:
            log.Printf("New client registered for table %s", t.Code)
            t.clients[client] = true
            
        case client := <-t.unregister:
            if _, ok := t.clients[client]; ok {
                log.Printf("Client unregistered from table %s", t.Code)
                delete(t.clients, client)
                close(client.send) // Assuming client has a 'send chan []byte'
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




