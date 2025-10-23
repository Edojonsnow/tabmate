package controllers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Be more restrictive in production
	},
	EnableCompression: true,
}

type TableClient struct {
    table    *Table       // Reference to the Table this client belongs to
    conn     *websocket.Conn
    send     chan []byte  // Buffered channel of outbound messages for this client
    userID   string       // The authenticated user ID
    username string       // The user's display name
}

type Message struct {
    SenderID string
    Content  string
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *TableClient) readPump() {
	defer func() {
		c.table.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		log.Printf("Received message from client %s: %s", c.userID, string(message))

		var msg struct {
            Type     string `json:"type"`
            Content  string `json:"content"`
            Username string `json:"username"`
            Item     string `json:"item"`
        }
        if err := json.Unmarshal(message, &msg); err != nil {
            log.Printf("Error parsing message: %v", err)
            continue
        }
        
        switch msg.Type {
        case "chat":
            msgStruct := Message{
                SenderID: c.username,
                Content:  msg.Content,
            }
            jsonMsg, _ := json.Marshal(msgStruct)
            for client := range c.table.clients {
                if client.userID != c.userID {
                    client.send <- jsonMsg
                }
            }
		case "user_joined":
            // Broadcast user joined event
            joinMsg := struct {
                Type     string `json:"type"`
                Username string `json:"username"`
            }{
                Type:     "user_joined",
                Username: msg.Username,
            }
            jsonJoinMsg, _ := json.Marshal(joinMsg)
            for client := range c.table.clients {
                    if client != c {     // ✅ don't send back to the one who joined
                        client.send <- jsonJoinMsg
                    }
            }

		case "user_disconnected":
			// Broadcast user disconnected event
			// 1️⃣ Remove client from connected list
			log.Printf("%s disconnected", msg.Username )
			delete(c.table.clients, c)

			// 2️⃣ Broadcast updated connected usernames to everyone
			usernames := c.table.GetUsernames()
			msg := struct {
				Type      string   `json:"type"`
				Usernames []string `json:"usernames"`
			}{
				Type:      "user_list",
				Usernames: usernames,
			}
			jsonMsg, _ := json.Marshal(msg)

			for client := range c.table.clients {
				client.send <- jsonMsg
			}

			// 3️⃣ Close connection
			c.conn.Close()

        case "menu_add":
            // Broadcast menu add event
            menuMsg := struct {
                Type     string `json:"type"`
                Username string `json:"username"`
                Item     string `json:"item"`
            }{
                Type:     "menu_add",
                Username: msg.Username,
                Item:     msg.Item,
            }
            jsonMenuMsg, _ := json.Marshal(menuMsg)
            for client := range c.table.clients {
                client.send <- jsonMenuMsg
            }
		case "menu_remove":
            // Broadcast menu remove event
            removeMsg := struct {
                Type     string `json:"type"`
                Username string `json:"username"`
                Item     string `json:"item"`
            }{
                Type:     "menu_remove",
                Username: msg.Username,
                Item:     msg.Item,
            }
            jsonRemoveMsg, _ := json.Marshal(removeMsg)
            for client := range c.table.clients {
                client.send <- jsonRemoveMsg
            }
		case "request_usernames":
			usernames := c.table.GetUsernames()
			usernamesMsg := struct {
				Type      string   `json:"type"`
				Usernames []string `json:"usernames"`
			}{
				Type:      "user_list",
				Usernames: usernames,
			}
			jsonMsg, err := json.Marshal(usernamesMsg)
			if err != nil {
				log.Printf("Failed to marshal usernames list: %v", err)
				continue
			}
			// Send back only to the requesting client
			c.send <- jsonMsg
        default:
            log.Printf("Unknown message type: %s", msg.Type)
        }
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *TableClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// w, err := c.conn.NextWriter(websocket.TextMessage)
			// if err != nil {
			// 	return
			// }
			// log.Printf("Broadcasting message to client %s: %s", c.userID, string(message))
			// w.Write(message)

			// Add queued chat messages to the current websocket message.
			// n := len(c.send)
			// for i := 0; i < n; i++ {
			// 	w.Write(newline)
			// 	w.Write(<-c.send)
			// }
			err := c.conn.WriteMessage(websocket.TextMessage, message)
            if err != nil {
                log.Printf("write error: %v", err)
                return
            }

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ServeWs(table *Table, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}

	// Create a temporary user ID for testing
	tempUserID := uuid.New().String()

	client := &TableClient{
		table:  table,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: tempUserID,
	}

	// Register the client
	table.register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

func ServeWsWithUser(table *Table, w http.ResponseWriter, r *http.Request, username string, email string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to websocket: %v", err)
		return
	}

	client := &TableClient{
		table:    table,
		conn:     conn,
		send:     make(chan []byte, 256),
		userID:   username, // Use username as userID
		username: username, // Store username for display
	}

	// Register the client
	table.register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

