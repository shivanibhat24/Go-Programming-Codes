package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yourusername/notes-sync-backend/internal/models"
	"github.com/yourusername/notes-sync-backend/pkg/clock"
	"github.com/yourusername/notes-sync-backend/pkg/crdt"
)

// Store handles data persistence
type Store struct {
	db *sql.DB
}

// NewStore creates a new store instance
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{db: db}
	if err := store.init(); err != nil {
		return nil, err
	}

	return store, nil
}

// init initializes the database schema
func (s *Store) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS notes (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		crdt_data TEXT NOT NULL,
		clock_data TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		deleted_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_notes_updated_at ON notes(updated_at);
	CREATE INDEX IF NOT EXISTS idx_notes_deleted_at ON notes(deleted_at);

	CREATE TABLE IF NOT EXISTS operations (
		id TEXT PRIMARY KEY,
		note_id TEXT NOT NULL,
		client_id TEXT NOT NULL,
		operation_data TEXT NOT NULL,
		clock_data TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (note_id) REFERENCES notes(id)
	);

	CREATE INDEX IF NOT EXISTS idx_operations_note_id ON operations(note_id);
	CREATE INDEX IF NOT EXISTS idx_operations_timestamp ON operations(timestamp);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// SaveNote saves or updates a note
func (s *Store) SaveNote(note *models.Note) error {
	crdtData, err := note.CRDT.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize CRDT: %w", err)
	}

	clockData, err := json.Marshal(note.Clock)
	if err != nil {
		return fmt.Errorf("failed to serialize clock: %w", err)
	}

	query := `
	INSERT INTO notes (id, title, content, crdt_data, clock_data, created_at, updated_at, deleted_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		content = excluded.content,
		crdt_data = excluded.crdt_data,
		clock_data = excluded.clock_data,
		updated_at = excluded.updated_at,
		deleted_at = excluded.deleted_at
	`

	var deletedAt interface{}
	if note.DeletedAt != nil {
		deletedAt = note.DeletedAt
	}

	_, err = s.db.Exec(query,
		note.ID,
		note.Title,
		note.Content,
		crdtData,
		string(clockData),
		note.CreatedAt,
		note.UpdatedAt,
		deletedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}

	return nil
}

// GetNote retrieves a note by ID
func (s *Store) GetNote(id string) (*models.Note, error) {
	query := `
	SELECT id, title, content, crdt_data, clock_data, created_at, updated_at, deleted_at
	FROM notes
	WHERE id = ?
	`

	var note models.Note
	var crdtData, clockData string
	var deletedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&note.ID,
		&note.Title,
		&note.Content,
		&crdtData,
		&clockData,
		&note.CreatedAt,
		&note.UpdatedAt,
		&deletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}

	// Deserialize CRDT
	crdtObj, err := crdt.FromJSON(crdtData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize CRDT: %w", err)
	}
	note.CRDT = crdtObj

	// Deserialize clock
	if err := json.Unmarshal([]byte(clockData), &note.Clock); err != nil {
		return nil, fmt.Errorf("failed to deserialize clock: %w", err)
	}

	if deletedAt.Valid {
		note.DeletedAt = &deletedAt.Time
	}

	return &note, nil
}

// GetAllNotes retrieves all non-deleted notes
func (s *Store) GetAllNotes() ([]models.Note, error) {
	query := `
	SELECT id, title, content, crdt_data, clock_data, created_at, updated_at, deleted_at
	FROM notes
	WHERE deleted_at IS NULL
	ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	var notes []models.Note
	for rows.Next() {
		var note models.Note
		var crdtData, clockData string
		var deletedAt sql.NullTime

		err := rows.Scan(
			&note.ID,
			&note.Title,
			&note.Content,
			&crdtData,
			&clockData,
			&note.CreatedAt,
			&note.UpdatedAt,
			&deletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan note: %w", err)
		}

		// Deserialize CRDT
		crdtObj, err := crdt.FromJSON(crdtData)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize CRDT: %w", err)
		}
		note.CRDT = crdtObj

		// Deserialize clock
		if err := json.Unmarshal([]byte(clockData), &note.Clock); err != nil {
			return nil, fmt.Errorf("failed to deserialize clock: %w", err)
		}

		if deletedAt.Valid {
			note.DeletedAt = &deletedAt.Time
		}

		notes = append(notes, note)
	}

	return notes, nil
}

// GetNotesModifiedSince retrieves notes modified after a given time
func (s *Store) GetNotesModifiedSince(since time.Time) ([]models.Note, error) {
	query := `
	SELECT id, title, content, crdt_data, clock_data, created_at, updated_at, deleted_at
	FROM notes
	WHERE updated_at > ?
	ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	var notes []models.Note
	for rows.Next() {
		var note models.Note
		var crdtData, clockData string
		var deletedAt sql.NullTime

		err := rows.Scan(
			&note.ID,
			&note.Title,
			&note.Content,
			&crdtData,
			&clockData,
			&note.CreatedAt,
			&note.UpdatedAt,
			&deletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan note: %w", err)
		}

		// Deserialize CRDT
		crdtObj, err := crdt.FromJSON(crdtData)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize CRDT: %w", err)
		}
		note.CRDT = crdtObj

		// Deserialize clock
		if err := json.Unmarshal([]byte(clockData), &note.Clock); err != nil {
			return nil, fmt.Errorf("failed to deserialize clock: %w", err)
		}

		if deletedAt.Valid {
			note.DeletedAt = &deletedAt.Time
		}

		notes = append(notes, note)
	}

	return notes, nil
}

// DeleteNote soft deletes a note
func (s *Store) DeleteNote(id string) error {
	query := `UPDATE notes SET deleted_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, time.Now(), id)
	return err
}

// SaveOperation saves an operation to the log
func (s *Store) SaveOperation(noteID string, op crdt.Operation) error {
	opData, err := json.Marshal(op)
	if err != nil {
		return fmt.Errorf("failed to serialize operation: %w", err)
	}

	clockData, err := json.Marshal(op.Clock)
	if err != nil {
		return fmt.Errorf("failed to serialize clock: %w", err)
	}

	query := `
	INSERT INTO operations (id, note_id, client_id, operation_data, clock_data, timestamp)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO NOTHING
	`

	_, err = s.db.Exec(query,
		op.ID,
		noteID,
		op.ClientID,
		string(opData),
		string(clockData),
		op.Timestamp,
	)

	return err
}

// GetOperationsForNote retrieves all operations for a note
func (s *Store) GetOperationsForNote(noteID string, since clock.VectorClock) ([]crdt.Operation, error) {
	query := `
	SELECT operation_data
	FROM operations
	WHERE note_id = ?
	ORDER BY timestamp ASC
	`

	rows, err := s.db.Query(query, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to query operations: %w", err)
	}
	defer rows.Close()

	var operations []crdt.Operation
	for rows.Next() {
		var opData string
		if err := rows.Scan(&opData); err != nil {
			return nil, fmt.Errorf("failed to scan operation: %w", err)
		}

		var op crdt.Operation
		if err := json.Unmarshal([]byte(opData), &op); err != nil {
			return nil, fmt.Errorf("failed to deserialize operation: %w", err)
		}

		// Filter by vector clock if provided
		if since != nil {
			if !op.Clock.HappensBefore(since) && op.Clock.Compare(since) != 0 {
				operations = append(operations, op)
			}
		} else {
			operations = append(operations, op)
		}
	}

	return operations, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}
