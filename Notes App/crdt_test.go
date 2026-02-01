package crdt

import (
	"testing"

	"github.com/yourusername/notes-sync-backend/pkg/clock"
)

func TestCRDTInsert(t *testing.T) {
	c := NewCRDT()
	
	// Insert "Hello"
	op1 := c.CreateInsertOperation("client1", 0, "Hello")
	err := c.ApplyOperation(op1)
	if err != nil {
		t.Fatalf("Failed to apply operation: %v", err)
	}
	
	if c.Text != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", c.Text)
	}
	
	// Insert " World" at position 5
	op2 := c.CreateInsertOperation("client1", 5, " World")
	err = c.ApplyOperation(op2)
	if err != nil {
		t.Fatalf("Failed to apply operation: %v", err)
	}
	
	if c.Text != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", c.Text)
	}
}

func TestCRDTDelete(t *testing.T) {
	c := NewCRDT()
	
	// Insert "Hello World"
	op1 := c.CreateInsertOperation("client1", 0, "Hello World")
	c.ApplyOperation(op1)
	
	// Delete character at position 5 (space)
	op2 := c.CreateDeleteOperation("client1", 5, 1)
	c.ApplyOperation(op2)
	
	if c.Text != "HelloWorld" {
		t.Errorf("Expected 'HelloWorld', got '%s'", c.Text)
	}
}

func TestCRDTConcurrentInserts(t *testing.T) {
	c1 := NewCRDT()
	c2 := NewCRDT()
	
	// Both start with "AB"
	op0 := c1.CreateInsertOperation("client1", 0, "AB")
	c1.ApplyOperation(op0)
	c2.ApplyOperation(op0)
	
	// Client 1 inserts "X" at position 1 -> "AXB"
	op1 := c1.CreateInsertOperation("client1", 1, "X")
	c1.ApplyOperation(op1)
	
	// Client 2 inserts "Y" at position 1 -> "AYB"
	op2 := c2.CreateInsertOperation("client2", 1, "Y")
	c2.ApplyOperation(op2)
	
	// Merge c2 into c1
	c1.ApplyOperation(op2)
	
	// Merge c1 into c2
	c2.ApplyOperation(op1)
	
	// Both should converge to the same state
	if c1.Text != c2.Text {
		t.Errorf("CRDTs did not converge: c1='%s', c2='%s'", c1.Text, c2.Text)
	}
}

func TestCRDTMerge(t *testing.T) {
	c1 := NewCRDT()
	c2 := NewCRDT()
	
	// Client 1 creates "Hello"
	op1 := c1.CreateInsertOperation("client1", 0, "Hello")
	c1.ApplyOperation(op1)
	
	// Client 2 creates "World"
	op2 := c2.CreateInsertOperation("client2", 0, "World")
	c2.ApplyOperation(op2)
	
	// Merge
	err := c1.Merge(c2)
	if err != nil {
		t.Fatalf("Failed to merge: %v", err)
	}
	
	// Both operations should be present
	if len(c1.Operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(c1.Operations))
	}
}

func TestCRDTGetOperationsSince(t *testing.T) {
	c := NewCRDT()
	
	// Apply several operations
	op1 := c.CreateInsertOperation("client1", 0, "A")
	c.ApplyOperation(op1)
	
	// Save clock state
	clock1 := c.Clock.Copy()
	
	op2 := c.CreateInsertOperation("client1", 1, "B")
	c.ApplyOperation(op2)
	
	op3 := c.CreateInsertOperation("client1", 2, "C")
	c.ApplyOperation(op3)
	
	// Get operations since clock1
	ops := c.GetOperationsSince(clock1)
	
	// Should get op2 and op3
	if len(ops) < 2 {
		t.Errorf("Expected at least 2 operations, got %d", len(ops))
	}
}

func TestCRDTSerializeDeserialize(t *testing.T) {
	c1 := NewCRDT()
	
	op := c1.CreateInsertOperation("client1", 0, "Test")
	c1.ApplyOperation(op)
	
	// Serialize
	json, err := c1.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	
	// Deserialize
	c2, err := FromJSON(json)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	
	if c2.Text != c1.Text {
		t.Errorf("Text mismatch: expected '%s', got '%s'", c1.Text, c2.Text)
	}
	
	if len(c2.Operations) != len(c1.Operations) {
		t.Errorf("Operations count mismatch: expected %d, got %d", 
			len(c1.Operations), len(c2.Operations))
	}
}

func TestCRDTOperationOrdering(t *testing.T) {
	c := NewCRDT()
	
	// Create operations with different clients and timestamps
	op1 := Operation{
		ID:       "op1",
		ClientID: "client1",
		Clock:    clock.VectorClock{"client1": 1},
		Type:     "insert",
		Position: 0,
		Content:  "A",
	}
	
	op2 := Operation{
		ID:       "op2",
		ClientID: "client2",
		Clock:    clock.VectorClock{"client2": 1},
		Type:     "insert",
		Position: 0,
		Content:  "B",
	}
	
	// Apply in different orders
	c1 := NewCRDT()
	c1.ApplyOperation(op1)
	c1.ApplyOperation(op2)
	
	c2 := NewCRDT()
	c2.ApplyOperation(op2)
	c2.ApplyOperation(op1)
	
	// Should converge
	if c1.Text != c2.Text {
		t.Errorf("Different ordering produced different results: c1='%s', c2='%s'", 
			c1.Text, c2.Text)
	}
}
