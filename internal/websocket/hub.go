package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now (configure for production)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message types for WebSocket events
type EventType string

const (
	EventCallStart    EventType = "call_start"
	EventCallUpdate   EventType = "call_update"
	EventCallEnd      EventType = "call_end"
	EventStatsUpdate  EventType = "stats_update"
	EventProjectStats EventType = "project_stats"
)

// Message represents a WebSocket message
type Message struct {
	Type      EventType   `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client represents a WebSocket client connection
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	topics map[string]bool // subscribed topics (e.g., "project:1", "all")
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// GlobalHub is the singleton hub instance
var GlobalHub *Hub

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Init initializes the global hub
func Init() {
	GlobalHub = NewHub()
	go GlobalHub.Run()
	log.Println("[WebSocket] Hub initialized")
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] Client connected. Total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] Client disconnected. Total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(eventType EventType, data interface{}) {
	msg := Message{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WebSocket] Error marshaling message: %v", err)
		return
	}

	h.broadcast <- jsonData
}

// BroadcastCallEvent broadcasts a call event to all clients
func BroadcastCallEvent(eventType EventType, callData interface{}) {
	if GlobalHub == nil {
		return
	}
	GlobalHub.Broadcast(eventType, callData)
}

// BroadcastStats broadcasts stats update to all clients
func BroadcastStats(stats interface{}) {
	if GlobalHub == nil {
		return
	}
	GlobalHub.Broadcast(EventStatsUpdate, stats)
}

// HandleWebSocket handles WebSocket upgrade requests
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:    GlobalHub,
		conn:   conn,
		send:   make(chan []byte, 256),
		topics: make(map[string]bool),
	}
	client.topics["all"] = true // Subscribe to all events by default

	GlobalHub.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection
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
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WebSocket] Read error: %v", err)
			}
			break
		}

		// Handle subscription messages (optional)
		var subMsg struct {
			Action string `json:"action"`
			Topic  string `json:"topic"`
		}
		if json.Unmarshal(message, &subMsg) == nil {
			if subMsg.Action == "subscribe" && subMsg.Topic != "" {
				c.topics[subMsg.Topic] = true
			} else if subMsg.Action == "unsubscribe" {
				delete(c.topics, subMsg.Topic)
			}
		}
	}
}

// writePump pumps messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
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

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
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

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
