// Package ws provides the WebSocket hub, client management, and event types.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // validated by auth middleware on upgrade
	},
}

// Event types sent over WebSocket.
const (
	EventServiceStatus   = "service.status"
	EventSyncProgress    = "sync.progress"
	EventSyncComplete    = "sync.complete"
	EventChangeDetected  = "change.detected"
	EventAlertCreated    = "alert.created"
	EventAlertResolved   = "alert.resolved"
	EventDocGenerated    = "doc.generated"
	EventDocAISuggestion = "doc.ai_suggestion"
	EventSystemHealth    = "system.health"
	EventSystemNotice    = "system.notice"
)

// Envelope wraps all WebSocket messages.
type Envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
	role   string
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan broadcastMsg
	register   chan *Client
	unregister chan *Client
}

type broadcastMsg struct {
	data   []byte
	userID string // empty = all clients
}

// NewHub creates a new WebSocket hub and starts its run loop.
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan broadcastMsg, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	return h
}

// Run starts the hub's event loop. Should be run in a goroutine.
func (h *Hub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			slog.Info("WebSocket client connected", "user_id", client.userID, "total_clients", count)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			count := len(h.clients)
			h.mu.Unlock()
			slog.Info("WebSocket client disconnected", "user_id", client.userID, "total_clients", count)

		case msg := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if msg.userID == "" || msg.userID == client.userID {
					select {
					case client.send <- msg.data:
					default:
						// Client's send buffer is full — drop message
						slog.Warn("WebSocket client send buffer full, dropping message", "user_id", client.userID)
					}
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			// Heartbeat
			heartbeat, _ := json.Marshal(Envelope{
				Type: EventSystemHealth,
				Payload: map[string]any{
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				},
			})
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- heartbeat:
				default:
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(eventType string, payload any) {
	data, err := json.Marshal(Envelope{Type: eventType, Payload: payload})
	if err != nil {
		slog.Error("failed to marshal WS broadcast", "error", err)
		return
	}
	h.broadcast <- broadcastMsg{data: data}
}

// BroadcastToUser sends a message to a specific user's connections.
func (h *Hub) BroadcastToUser(userID, eventType string, payload any) {
	data, err := json.Marshal(Envelope{Type: eventType, Payload: payload})
	if err != nil {
		slog.Error("failed to marshal WS broadcast", "error", err)
		return
	}
	h.broadcast <- broadcastMsg{data: data, userID: userID}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// UpgradeHandler upgrades an HTTP connection to WebSocket.
// Caller must authenticate before upgrading.
func (h *Hub) UpgradeHandler(w http.ResponseWriter, r *http.Request, userID, role string) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	client := &Client{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		role:   role,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()

	return nil
}

// readPump reads messages from the WebSocket connection (pings/pongs only).
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
