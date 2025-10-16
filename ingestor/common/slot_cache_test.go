package common

import (
	"testing"
	"time"
)

func TestMemorySlotTimeCache_SetAndGet(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	slot := uint64(12345)
	timestamp := time.Now()

	cache.Set(slot, timestamp)

	retrieved, err := cache.Get(slot)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !retrieved.Equal(timestamp) {
		t.Errorf("Expected timestamp %v, got %v", timestamp, retrieved)
	}
}

func TestMemorySlotTimeCache_GetNonExistent(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	_, err := cache.Get(99999)
	if err == nil {
		t.Error("Expected error for non-existent slot, got nil")
	}
}

func TestMemorySlotTimeCache_GetRange(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	// Set up test data
	baseTime := time.Now()
	cache.Set(100, baseTime)
	cache.Set(101, baseTime.Add(time.Second))
	cache.Set(102, baseTime.Add(2*time.Second))
	cache.Set(105, baseTime.Add(5*time.Second)) // Gap at 103, 104

	// Test range query
	result := cache.GetRange(100, 105)

	if len(result) != 4 {
		t.Errorf("Expected 4 results, got %d", len(result))
	}

	// Verify specific slots
	if _, ok := result[100]; !ok {
		t.Error("Expected slot 100 in results")
	}
	if _, ok := result[103]; ok {
		t.Error("Did not expect slot 103 in results")
	}
	if _, ok := result[105]; !ok {
		t.Error("Expected slot 105 in results")
	}
}

func TestMemorySlotTimeCache_ReplayMarker(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	// Initially, replay marker should be 0
	if marker := cache.GetReplayMarker(); marker != 0 {
		t.Errorf("Expected initial replay marker to be 0, got %d", marker)
	}

	// Set replay marker
	replaySlot := uint64(5000)
	cache.SetReplayMarker(replaySlot)

	if marker := cache.GetReplayMarker(); marker != replaySlot {
		t.Errorf("Expected replay marker %d, got %d", replaySlot, marker)
	}

	// Test IsReplaySlot
	testCases := []struct {
		slot     uint64
		expected bool
	}{
		{4999, false}, // Before marker
		{5000, true},  // At marker
		{5001, true},  // After marker
	}

	for _, tc := range testCases {
		result := cache.IsReplaySlot(tc.slot)
		if result != tc.expected {
			t.Errorf("IsReplaySlot(%d): expected %v, got %v", tc.slot, tc.expected, result)
		}
	}
}

func TestMemorySlotTimeCache_Clear(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	// Add some data
	cache.Set(100, time.Now())
	cache.Set(101, time.Now())
	cache.SetReplayMarker(100)

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	// Clear cache
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}

	if marker := cache.GetReplayMarker(); marker != 0 {
		t.Errorf("Expected replay marker 0 after clear, got %d", marker)
	}
}

func TestMemorySlotTimeCache_Size(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	if cache.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", cache.Size())
	}

	cache.Set(1, time.Now())
	cache.Set(2, time.Now())
	cache.Set(3, time.Now())

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	// Setting same slot shouldn't increase size
	cache.Set(1, time.Now())

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after duplicate set, got %d", cache.Size())
	}
}

func TestMemorySlotTimeCache_PruneBeforeSlot(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	// Add slots 100-110
	baseTime := time.Now()
	for i := uint64(100); i <= 110; i++ {
		cache.Set(i, baseTime.Add(time.Duration(i)*time.Second))
	}

	if cache.Size() != 11 {
		t.Errorf("Expected initial size 11, got %d", cache.Size())
	}

	// Prune slots before 105
	pruned := cache.PruneBeforeSlot(105)

	if pruned != 5 {
		t.Errorf("Expected 5 pruned entries, got %d", pruned)
	}

	if cache.Size() != 6 {
		t.Errorf("Expected size 6 after pruning, got %d", cache.Size())
	}

	// Verify pruned slots are gone
	_, err := cache.Get(104)
	if err == nil {
		t.Error("Expected slot 104 to be pruned")
	}

	// Verify remaining slots are still present
	_, err = cache.Get(105)
	if err != nil {
		t.Error("Expected slot 105 to still be present")
	}

	_, err = cache.Get(110)
	if err != nil {
		t.Error("Expected slot 110 to still be present")
	}
}

func TestMemorySlotTimeCache_Concurrent(t *testing.T) {
	cache := NewMemorySlotTimeCache()

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := uint64(0); i < 1000; i++ {
			cache.Set(i, time.Now())
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := uint64(0); i < 1000; i++ {
			_, _ = cache.Get(i) // Ignore errors, just test for races
		}
		done <- true
	}()

	// Pruner goroutine
	go func() {
		for i := uint64(0); i < 100; i++ {
			cache.PruneBeforeSlot(i * 10)
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// If we reach here without data races, test passes
}
