package common

import (
	"fmt"
	"sync"
	"time"
)

// SlotTimeCache provides a thread-safe mapping of Solana slot numbers to timestamps
// with support for replay markers to track reprocessing boundaries
type SlotTimeCache interface {
	// Get retrieves the timestamp for a given slot
	// Returns an error if the slot is not in the cache
	Get(slot uint64) (time.Time, error)

	// Set stores a slot-to-timestamp mapping
	Set(slot uint64, timestamp time.Time)

	// GetRange retrieves timestamps for a range of slots [startSlot, endSlot]
	// Returns a map of slot -> timestamp for slots found in the cache
	GetRange(startSlot, endSlot uint64) map[uint64]time.Time

	// SetReplayMarker marks a slot as the replay boundary
	// This indicates that slots >= this value are being replayed/reprocessed
	SetReplayMarker(slot uint64)

	// GetReplayMarker returns the current replay marker slot
	GetReplayMarker() uint64

	// IsReplaySlot returns true if the given slot is at or after the replay marker
	IsReplaySlot(slot uint64) bool

	// Clear removes all entries from the cache
	Clear()

	// Size returns the number of entries in the cache
	Size() int

	// PruneBeforeSlot removes all entries with slot numbers less than the given slot
	PruneBeforeSlot(slot uint64) int
}

// MemorySlotTimeCache is an in-memory implementation of SlotTimeCache
type MemorySlotTimeCache struct {
	mu           sync.RWMutex
	slots        map[uint64]time.Time
	replayMarker uint64
}

// NewMemorySlotTimeCache creates a new in-memory slot time cache
func NewMemorySlotTimeCache() SlotTimeCache {
	return &MemorySlotTimeCache{
		slots:        make(map[uint64]time.Time),
		replayMarker: 0,
	}
}

// Get retrieves the timestamp for a given slot
func (c *MemorySlotTimeCache) Get(slot uint64) (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ts, ok := c.slots[slot]
	if !ok {
		return time.Time{}, fmt.Errorf("slot %d not found in cache", slot)
	}
	return ts, nil
}

// Set stores a slot-to-timestamp mapping
func (c *MemorySlotTimeCache) Set(slot uint64, timestamp time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slots[slot] = timestamp
}

// GetRange retrieves timestamps for a range of slots
func (c *MemorySlotTimeCache) GetRange(startSlot, endSlot uint64) map[uint64]time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[uint64]time.Time)
	for slot := startSlot; slot <= endSlot; slot++ {
		if ts, ok := c.slots[slot]; ok {
			result[slot] = ts
		}
	}
	return result
}

// SetReplayMarker marks a slot as the replay boundary
func (c *MemorySlotTimeCache) SetReplayMarker(slot uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.replayMarker = slot
}

// GetReplayMarker returns the current replay marker slot
func (c *MemorySlotTimeCache) GetReplayMarker() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.replayMarker
}

// IsReplaySlot returns true if the given slot is at or after the replay marker
func (c *MemorySlotTimeCache) IsReplaySlot(slot uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.replayMarker > 0 && slot >= c.replayMarker
}

// Clear removes all entries from the cache
func (c *MemorySlotTimeCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slots = make(map[uint64]time.Time)
	c.replayMarker = 0
}

// Size returns the number of entries in the cache
func (c *MemorySlotTimeCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.slots)
}

// PruneBeforeSlot removes all entries with slot numbers less than the given slot
func (c *MemorySlotTimeCache) PruneBeforeSlot(slot uint64) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	pruned := 0
	for s := range c.slots {
		if s < slot {
			delete(c.slots, s)
			pruned++
		}
	}
	return pruned
}
