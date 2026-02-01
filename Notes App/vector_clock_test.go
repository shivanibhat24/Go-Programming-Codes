package clock

import (
	"testing"
)

func TestVectorClockIncrement(t *testing.T) {
	vc := NewVectorClock()
	vc.Increment("client1")
	
	if vc["client1"] != 1 {
		t.Errorf("Expected client1 to be 1, got %d", vc["client1"])
	}
	
	vc.Increment("client1")
	if vc["client1"] != 2 {
		t.Errorf("Expected client1 to be 2, got %d", vc["client1"])
	}
}

func TestVectorClockUpdate(t *testing.T) {
	vc1 := NewVectorClock()
	vc1["client1"] = 5
	vc1["client2"] = 3
	
	vc2 := NewVectorClock()
	vc2["client1"] = 3
	vc2["client2"] = 7
	vc2["client3"] = 2
	
	vc1.Update(vc2)
	
	if vc1["client1"] != 5 {
		t.Errorf("Expected client1 to be 5, got %d", vc1["client1"])
	}
	if vc1["client2"] != 7 {
		t.Errorf("Expected client2 to be 7, got %d", vc1["client2"])
	}
	if vc1["client3"] != 2 {
		t.Errorf("Expected client3 to be 2, got %d", vc1["client3"])
	}
}

func TestVectorClockCompare(t *testing.T) {
	tests := []struct {
		name     string
		vc1      VectorClock
		vc2      VectorClock
		expected int
	}{
		{
			name:     "vc1 before vc2",
			vc1:      VectorClock{"client1": 1, "client2": 2},
			vc2:      VectorClock{"client1": 2, "client2": 3},
			expected: -1,
		},
		{
			name:     "vc1 after vc2",
			vc1:      VectorClock{"client1": 5, "client2": 5},
			vc2:      VectorClock{"client1": 3, "client2": 4},
			expected: 1,
		},
		{
			name:     "concurrent",
			vc1:      VectorClock{"client1": 5, "client2": 2},
			vc2:      VectorClock{"client1": 3, "client2": 7},
			expected: 0,
		},
		{
			name:     "equal",
			vc1:      VectorClock{"client1": 3, "client2": 4},
			vc2:      VectorClock{"client1": 3, "client2": 4},
			expected: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.vc1.Compare(tt.vc2)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestVectorClockCopy(t *testing.T) {
	vc := NewVectorClock()
	vc["client1"] = 5
	
	copied := vc.Copy()
	copied["client1"] = 10
	
	if vc["client1"] != 5 {
		t.Errorf("Original should not be modified, got %d", vc["client1"])
	}
	if copied["client1"] != 10 {
		t.Errorf("Copy should be modified, got %d", copied["client1"])
	}
}

func TestVectorClockHappensBefore(t *testing.T) {
	vc1 := VectorClock{"client1": 1, "client2": 2}
	vc2 := VectorClock{"client1": 2, "client2": 3}
	
	if !vc1.HappensBefore(vc2) {
		t.Error("vc1 should happen before vc2")
	}
	
	if vc2.HappensBefore(vc1) {
		t.Error("vc2 should not happen before vc1")
	}
}

func TestVectorClockIsConcurrent(t *testing.T) {
	vc1 := VectorClock{"client1": 5, "client2": 2}
	vc2 := VectorClock{"client1": 3, "client2": 7}
	
	if !vc1.IsConcurrent(vc2) {
		t.Error("vc1 and vc2 should be concurrent")
	}
	
	vc3 := VectorClock{"client1": 1, "client2": 1}
	vc4 := VectorClock{"client1": 2, "client2": 2}
	
	if vc3.IsConcurrent(vc4) {
		t.Error("vc3 and vc4 should not be concurrent")
	}
}
