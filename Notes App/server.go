package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yourusername/notes-sync-backend/internal/models"
	"github.com/yourusername/notes-sync-backend/internal/sync"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Server represents the API server
type Server struct {
	syncEngine *sync.Engine
	clients    map[string]*Client
	clientsMux sync.RWMutex
	broadcast  chan *models.WebSocketMessage
}

// Client represents a connected WebSocket client
type Client struct {
	ID     string
	conn   *websocket.Conn
	send   chan *models.WebSocketMessage
	server *Server
}

// NewServer creates a new API server
func NewServer(engine *sync.Engine) *Server {
	return &Server{
		syncEngine: engine,
		clients:    make(map[string]*Client),
		broadcast:  make(chan *models.WebSocketMessage, 256),
	}
}

// Start starts the broadcast handler
func (s *Server) Start() {
	go s.handleBroadcast()
}

// handleBroadcast handles broadcasting messages to all clients
func (s *Server) handleBroadcast() {
	for msg := range s.broadcast {
		s.clientsMux.RLock()
		for _, client := range s.clients {
			select {
			case client.send <- msg:
			default:
				close(client.send)
				delete(s.clients, client.ID)
			}
		}
		s.clientsMux.RUnlock()
	}
}

// HandleSync handles HTTP sync requests
func (s *Server) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate client ID
	if req.ClientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Process sync
	resp, err := s.syncEngine.Sync(req)
	if err != nil {
		log.Printf("Sync error: %v", err)
		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleGetNotes handles getting all notes
func (s *Server) HandleGetNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	notes, err := s.syncEngine.GetAllNotes()
	if err != nil {
		log.Printf("Get notes error: %v", err)
		http.Error(w, "Failed to get notes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

// HandleCreateNote handles creating a new note
func (s *Server) HandleCreateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		ClientID string `json:"client_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// Generate UUID for the note
	noteID := uuid.New().String()

	note, err := s.syncEngine.CreateNote(noteID, req.Title, req.Content, req.ClientID)
	if err != nil {
		log.Printf("Create note error: %v", err)
		http.Error(w, "Failed to create note", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(note)
}

// HandleWebSocket handles WebSocket connections for real-time sync
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = uuid.New().String()
	}

	client := &Client{
		ID:     clientID,
		conn:   conn,
		send:   make(chan *models.WebSocketMessage, 256),
		server: s,
	}

	s.clientsMux.Lock()
	s.clients[clientID] = client
	s.clientsMux.Unlock()

	// Start goroutines for this client
	go client.writePump()
	go client.readPump()

	log.Printf("Client connected: %s", clientID)
}

// readPump reads messages from the WebSocket
func (c *Client) readPump() {
	defer func() {
		c.server.clientsMux.Lock()
		delete(c.server.clients, c.ID)
		c.server.clientsMux.Unlock()
		c.conn.Close()
		log.Printf("Client disconnected: %s", c.ID)
	}()

	for {
		var msg models.WebSocketMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle different message types
		switch msg.Type {
		case "edit":
			c.handleEdit(msg.Payload)
		case "sync":
			c.handleSyncRequest(msg.Payload)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// writePump writes messages to the WebSocket
func (c *Client) writePump() {
	defer c.conn.Close()

	for msg := range c.send {
		if err := c.conn.WriteJSON(msg); err != nil {
			log.Printf("Write error: %v", err)
			return
		}
	}
}

// handleEdit handles real-time edit operations
func (c *Client) handleEdit(payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal edit payload: %v", err)
		return
	}

	var edit models.EditOperation
	if err := json.Unmarshal(data, &edit); err != nil {
		log.Printf("Failed to unmarshal edit operation: %v", err)
		return
	}

	// Apply the operation
	if err := c.server.syncEngine.ApplyOperation(edit.NoteID, edit.ClientID, edit.Operation); err != nil {
		log.Printf("Failed to apply operation: %v", err)
		return
	}

	// Broadcast to other clients
	c.server.broadcast <- &models.WebSocketMessage{
		Type:    "edit",
		Payload: edit,
	}

	log.Printf("Applied edit for note %s from client %s", edit.NoteID, edit.ClientID)
}

// handleSyncRequest handles sync requests over WebSocket
func (c *Client) handleSyncRequest(payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal sync payload: %v", err)
		return
	}

	var req models.SyncRequest
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("Failed to unmarshal sync request: %v", err)
		return
	}

	resp, err := c.server.syncEngine.Sync(req)
	if err != nil {
		log.Printf("Sync error: %v", err)
		return
	}

	// Send response back to client
	c.send <- &models.WebSocketMessage{
		Type:    "sync_response",
		Payload: resp,
	}
}

// EnableCORS adds CORS headers to responses
func EnableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
