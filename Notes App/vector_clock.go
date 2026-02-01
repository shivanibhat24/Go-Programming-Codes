package clock

import (
	"encoding/json"
	"fmt"
)

// VectorClock represents a logical clock for tracking causality
type VectorClock map[string]int64

// NewVectorClock creates a new vector clock
func NewVectorClock() VectorClock {
	return make(VectorClock)
}

// Increment increments the clock for a given client
func (vc VectorClock) Increment(clientID string) {
	vc[clientID]++
}

// Update merges another vector clock, taking the maximum of each component
func (vc VectorClock) Update(other VectorClock) {
	for clientID, timestamp := range other {
		if vc[clientID] < timestamp {
			vc[clientID] = timestamp
		}
	}
}

// Copy creates a deep copy of the vector clock
func (vc VectorClock) Copy() VectorClock {
	newVC := make(VectorClock, len(vc))
	for k, v := range vc {
		newVC[k] = v
	}
	return newVC
}

// Compare compares two vector clocks and returns the relationship
// Returns: -1 (vc < other), 0 (concurrent), 1 (vc > other)
func (vc VectorClock) Compare(other VectorClock) int {
	hasLess := false
	hasGreater := false

	// Check all keys in both clocks
	allKeys := make(map[string]bool)
	for k := range vc {
		allKeys[k] = true
	}
	for k := range other {
		allKeys[k] = true
	}

	for k := range allKeys {
		v1 := vc[k]
		v2 := other[k]

		if v1 < v2 {
			hasLess = true
		} else if v1 > v2 {
			hasGreater = true
		}
	}

	if hasLess && hasGreater {
		return 0 // Concurrent
	} else if hasLess {
		return -1 // vc < other
	} else if hasGreater {
		return 1 // vc > other
	}
	return 0 // Equal (treated as concurrent)
}

// IsConcurrent checks if two vector clocks are concurrent
func (vc VectorClock) IsConcurrent(other VectorClock) bool {
	return vc.Compare(other) == 0
}

// HappensBefore checks if this clock happens before another
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	return vc.Compare(other) == -1
}

// String returns a string representation
func (vc VectorClock) String() string {
	data, _ := json.Marshal(vc)
	return string(data)
}

// ParseVectorClock parses a vector clock from JSON string
func ParseVectorClock(s string) (VectorClock, error) {
	var vc VectorClock
	if err := json.Unmarshal([]byte(s), &vc); err != nil {
		return nil, fmt.Errorf("failed to parse vector clock: %w", err)
	}
	return vc, nil
}
