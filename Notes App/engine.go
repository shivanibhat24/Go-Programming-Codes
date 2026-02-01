package sync

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/notes-sync-backend/internal/models"
	"github.com/yourusername/notes-sync-backend/internal/store"
	"github.com/yourusername/notes-sync-backend/pkg/clock"
	"github.com/yourusername/notes-sync-backend/pkg/crdt"
)

// Engine handles synchronization logic
type Engine struct {
	store *store.Store
}

// NewEngine creates a new sync engine
func NewEngine(s *store.Store) *Engine {
	return &Engine{
		store: s,
	}
}

// Sync processes a sync request from a client
func (e *Engine) Sync(req models.SyncRequest) (*models.SyncResponse, error) {
	response := &models.SyncResponse{
		Notes:     make([]models.Note, 0),
		Conflicts: make([]models.Conflict, 0),
		Clock:     make(map[string]clock.VectorClock),
	}

	// Process each note from the client
	for _, clientNote := range req.Notes {
		conflict, err := e.syncNote(&clientNote, req.ClientID, req.LastSync[clientNote.ID])
		if err != nil {
			log.Printf("Error syncing note %s: %v", clientNote.ID, err)
			continue
		}

		if conflict != nil {
			response.Conflicts = append(response.Conflicts, *conflict)
		}
	}

	// Get all server notes to send to client
	serverNotes, err := e.store.GetAllNotes()
	if err != nil {
		return nil, fmt.Errorf("failed to get server notes: %w", err)
	}

	// Filter notes based on client's last sync
	for _, serverNote := range serverNotes {
		lastSync := req.LastSync[serverNote.ID]
		
		// If client has never synced this note, or server has newer changes
		if lastSync == nil || serverNote.Clock.Compare(lastSync) == 1 {
			response.Notes = append(response.Notes, serverNote)
			response.Clock[serverNote.ID] = serverNote.Clock
		}
	}

	return response, nil
}

// syncNote syncs a single note and handles conflicts
func (e *Engine) syncNote(clientNote *models.Note, clientID string, lastSync clock.VectorClock) (*models.Conflict, error) {
	// Get the server version of the note
	serverNote, err := e.store.GetNote(clientNote.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server note: %w", err)
	}

	// If note doesn't exist on server, just save it
	if serverNote == nil {
		clientNote.UpdatedAt = time.Now()
		if err := e.store.SaveNote(clientNote); err != nil {
			return nil, fmt.Errorf("failed to save new note: %w", err)
		}
		return nil, nil
	}

	// Check for conflicts using vector clocks
	comparison := clientNote.Clock.Compare(serverNote.Clock)

	switch comparison {
	case -1:
		// Client is behind, server wins
		log.Printf("Client behind for note %s, server wins", clientNote.ID)
		return nil, nil

	case 1:
		// Server is behind, client wins
		log.Printf("Server behind for note %s, client wins", clientNote.ID)
		serverNote.CRDT.Merge(clientNote.CRDT)
		serverNote.Clock.Update(clientNote.Clock)
		serverNote.Content = serverNote.CRDT.Text
		serverNote.UpdatedAt = time.Now()
		
		if err := e.store.SaveNote(serverNote); err != nil {
			return nil, fmt.Errorf("failed to update note: %w", err)
		}
		
		return nil, nil

	case 0:
		// Concurrent edits - merge with CRDT
		log.Printf("Concurrent edits for note %s, merging with CRDT", clientNote.ID)
		
		// Merge the CRDTs
		if err := serverNote.CRDT.Merge(clientNote.CRDT); err != nil {
			return nil, fmt.Errorf("failed to merge CRDTs: %w", err)
		}

		// Update the clock
		serverNote.Clock.Update(clientNote.Clock)
		
		// Update content from merged CRDT
		serverNote.Content = serverNote.CRDT.Text
		serverNote.UpdatedAt = time.Now()

		// Save merged note
		if err := e.store.SaveNote(serverNote); err != nil {
			return nil, fmt.Errorf("failed to save merged note: %w", err)
		}

		// Return conflict info
		conflict := &models.Conflict{
			NoteID:      clientNote.ID,
			ClientClock: clientNote.Clock,
			ServerClock: serverNote.Clock,
			Resolution:  "merged",
		}

		return conflict, nil
	}

	return nil, nil
}

// ApplyOperation applies a real-time operation to a note
func (e *Engine) ApplyOperation(noteID, clientID string, op crdt.Operation) error {
	// Get the note
	note, err := e.store.GetNote(noteID)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", err)
	}

	if note == nil {
		return fmt.Errorf("note not found: %s", noteID)
	}

	// Apply the operation to the CRDT
	if err := note.CRDT.ApplyOperation(op); err != nil {
		return fmt.Errorf("failed to apply operation: %w", err)
	}

	// Update the note
	note.Clock.Update(op.Clock)
	note.Content = note.CRDT.Text
	note.UpdatedAt = time.Now()

	// Save the note
	if err := e.store.SaveNote(note); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	// Save the operation
	if err := e.store.SaveOperation(noteID, op); err != nil {
		return fmt.Errorf("failed to save operation: %w", err)
	}

	return nil
}

// GetNote retrieves a note by ID
func (e *Engine) GetNote(noteID string) (*models.Note, error) {
	return e.store.GetNote(noteID)
}

// CreateNote creates a new note
func (e *Engine) CreateNote(id, title, content, clientID string) (*models.Note, error) {
	note := models.NewNote(id, title, content)
	note.Clock.Increment(clientID)
	
	// Initialize CRDT with content
	if content != "" {
		op := note.CRDT.CreateInsertOperation(clientID, 0, content)
		if err := note.CRDT.ApplyOperation(op); err != nil {
			return nil, fmt.Errorf("failed to initialize CRDT: %w", err)
		}
	}

	if err := e.store.SaveNote(note); err != nil {
		return nil, fmt.Errorf("failed to save note: %w", err)
	}

	return note, nil
}

// DeleteNote deletes a note
func (e *Engine) DeleteNote(noteID string) error {
	return e.store.DeleteNote(noteID)
}

// GetAllNotes retrieves all notes
func (e *Engine) GetAllNotes() ([]models.Note, error) {
	return e.store.GetAllNotes()
}
