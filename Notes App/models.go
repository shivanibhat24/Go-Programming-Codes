package models

import (
	"time"

	"github.com/yourusername/notes-sync-backend/pkg/clock"
	"github.com/yourusername/notes-sync-backend/pkg/crdt"
)

// Note represents a single note
type Note struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	CRDT      *crdt.CRDT        `json:"crdt"`
	Clock     clock.VectorClock `json:"clock"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	DeletedAt *time.Time        `json:"deleted_at,omitempty"`
}

// NewNote creates a new note
func NewNote(id, title, content string) *Note {
	now := time.Now()
	c := crdt.NewCRDT()
	
	return &Note{
		ID:        id,
		Title:     title,
		Content:   content,
		CRDT:      c,
		Clock:     clock.NewVectorClock(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SyncRequest represents a sync request from a client
type SyncRequest struct {
	ClientID  string                       `json:"client_id"`
	Notes     []Note                       `json:"notes"`
	LastSync  map[string]clock.VectorClock `json:"last_sync"` // note_id -> vector clock
}

// SyncResponse represents a sync response to a client
type SyncResponse struct {
	Notes     []Note                       `json:"notes"`
	Conflicts []Conflict                   `json:"conflicts"`
	Clock     map[string]clock.VectorClock `json:"clock"` // note_id -> vector clock
}

// Conflict represents a detected conflict
type Conflict struct {
	NoteID       string `json:"note_id"`
	ClientClock  clock.VectorClock `json:"client_clock"`
	ServerClock  clock.VectorClock `json:"server_clock"`
	Resolution   string `json:"resolution"` // "merged", "client_wins", "server_wins"
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// EditOperation represents a real-time edit
type EditOperation struct {
	NoteID    string          `json:"note_id"`
	ClientID  string          `json:"client_id"`
	Operation crdt.Operation  `json:"operation"`
}
