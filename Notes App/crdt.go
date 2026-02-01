package crdt

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/yourusername/notes-sync-backend/pkg/clock"
)

// Operation represents a single edit operation
type Operation struct {
	ID        string              `json:"id"`
	ClientID  string              `json:"client_id"`
	Clock     clock.VectorClock   `json:"clock"`
	Type      string              `json:"type"` // "insert" or "delete"
	Position  int                 `json:"position"`
	Content   string              `json:"content,omitempty"`
	Timestamp time.Time           `json:"timestamp"`
}

// CRDT represents a Conflict-free Replicated Data Type for text
type CRDT struct {
	Operations []Operation       `json:"operations"`
	Text       string            `json:"text"`
	Clock      clock.VectorClock `json:"clock"`
}

// NewCRDT creates a new CRDT instance
func NewCRDT() *CRDT {
	return &CRDT{
		Operations: make([]Operation, 0),
		Text:       "",
		Clock:      clock.NewVectorClock(),
	}
}

// ApplyOperation applies an operation to the CRDT
func (c *CRDT) ApplyOperation(op Operation) error {
	// Update vector clock
	c.Clock.Update(op.Clock)
	
	// Add to operations log
	c.Operations = append(c.Operations, op)
	
	// Rebuild text from operations
	c.rebuildText()
	
	return nil
}

// ApplyOperations applies multiple operations
func (c *CRDT) ApplyOperations(ops []Operation) error {
	for _, op := range ops {
		if err := c.ApplyOperation(op); err != nil {
			return err
		}
	}
	return nil
}

// CreateInsertOperation creates a new insert operation
func (c *CRDT) CreateInsertOperation(clientID string, position int, content string) Operation {
	c.Clock.Increment(clientID)
	
	return Operation{
		ID:        generateOperationID(clientID, c.Clock),
		ClientID:  clientID,
		Clock:     c.Clock.Copy(),
		Type:      "insert",
		Position:  position,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// CreateDeleteOperation creates a new delete operation
func (c *CRDT) CreateDeleteOperation(clientID string, position int, length int) Operation {
	c.Clock.Increment(clientID)
	
	return Operation{
		ID:        generateOperationID(clientID, c.Clock),
		ClientID:  clientID,
		Clock:     c.Clock.Copy(),
		Type:      "delete",
		Position:  position,
		Content:   "",
		Timestamp: time.Now(),
	}
}

// rebuildText reconstructs the text from all operations
func (c *CRDT) rebuildText() {
	// Sort operations by timestamp and vector clock
	sortedOps := make([]Operation, len(c.Operations))
	copy(sortedOps, c.Operations)
	
	sort.Slice(sortedOps, func(i, j int) bool {
		// First compare by vector clock
		cmp := sortedOps[i].Clock.Compare(sortedOps[j].Clock)
		if cmp == -1 {
			return true
		} else if cmp == 1 {
			return false
		}
		// If concurrent, use timestamp as tiebreaker
		if !sortedOps[i].Timestamp.Equal(sortedOps[j].Timestamp) {
			return sortedOps[i].Timestamp.Before(sortedOps[j].Timestamp)
		}
		// Final tiebreaker: client ID
		return sortedOps[i].ClientID < sortedOps[j].ClientID
	})
	
	// Rebuild text
	text := []rune{}
	
	for _, op := range sortedOps {
		switch op.Type {
		case "insert":
			if op.Position <= len(text) {
				// Insert content at position
				content := []rune(op.Content)
				text = append(text[:op.Position], append(content, text[op.Position:]...)...)
			}
		case "delete":
			if op.Position < len(text) {
				// Delete one character at position
				text = append(text[:op.Position], text[op.Position+1:]...)
			}
		}
	}
	
	c.Text = string(text)
}

// GetOperationsSince returns operations after a given vector clock
func (c *CRDT) GetOperationsSince(since clock.VectorClock) []Operation {
	var ops []Operation
	
	for _, op := range c.Operations {
		// If the operation's clock is not before or equal to 'since', include it
		if !op.Clock.HappensBefore(since) && op.Clock.Compare(since) != 0 {
			ops = append(ops, op)
		}
	}
	
	return ops
}

// Merge merges another CRDT into this one
func (c *CRDT) Merge(other *CRDT) error {
	// Get operations we don't have
	newOps := make([]Operation, 0)
	
	existingIDs := make(map[string]bool)
	for _, op := range c.Operations {
		existingIDs[op.ID] = true
	}
	
	for _, op := range other.Operations {
		if !existingIDs[op.ID] {
			newOps = append(newOps, op)
		}
	}
	
	// Apply new operations
	return c.ApplyOperations(newOps)
}

// ToJSON serializes the CRDT to JSON
func (c *CRDT) ToJSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON deserializes a CRDT from JSON
func FromJSON(jsonStr string) (*CRDT, error) {
	var c CRDT
	if err := json.Unmarshal([]byte(jsonStr), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// generateOperationID generates a unique operation ID
func generateOperationID(clientID string, vc clock.VectorClock) string {
	return clientID + "-" + vc.String()
}
