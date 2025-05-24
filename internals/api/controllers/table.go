package controllers

import (
	"context"
	"log"
	tablesclea "tabmate/internals/store/postgres"

	"github.com/google/uuid"
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
	codes, err := db.GetAllTableCodes(ctx)
	if err != nil {
		return err
	}

	for _, code := range codes {
		if _, exists := activeTables[code]; !exists {
			activeTables[code] = nil 
		}
	}
	return nil
}

func GetTable(code string) *Table {
	if table, exists := activeTables[code]; exists {
		return table
	}
	return nil
}

func CreateTable() *Table {

	newTableCode := uuid.New().String()[:8]
	newTable := NewTable(uuid.New(), newTableCode)
	activeTables[newTableCode] = newTable
	

	go newTable.Run()
	
	return newTable
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




