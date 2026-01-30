package dialer

import (
	"log"
	"sync"
	"sync/atomic"
)

// ChannelPool manages concurrent call limits
// It tracks active channels globally and per-trunk to prevent system overload
type ChannelPool struct {
	maxGlobal      int32            // Maximum global concurrent calls
	maxPerTrunk    int32            // Maximum calls per trunk
	activeGlobal   int32            // Current global active calls (atomic)
	perTrunk       sync.Map         // trunk -> *int32 (atomic counter)
	mu             sync.RWMutex
}

// NewChannelPool creates a new channel pool with specified limits
func NewChannelPool(maxGlobal, maxPerTrunk int) *ChannelPool {
	return &ChannelPool{
		maxGlobal:   int32(maxGlobal),
		maxPerTrunk: int32(maxPerTrunk),
	}
}

// Acquire attempts to acquire a channel slot for the given trunk
// Returns true if successful, false if limits would be exceeded
func (cp *ChannelPool) Acquire(trunk string) bool {
	// Check global limit first
	current := atomic.LoadInt32(&cp.activeGlobal)
	if current >= cp.maxGlobal {
		log.Printf("[ChannelPool] Global limit reached: %d/%d", current, cp.maxGlobal)
		return false
	}

	// Get or create per-trunk counter
	counterI, _ := cp.perTrunk.LoadOrStore(trunk, new(int32))
	counter := counterI.(*int32)

	// Check per-trunk limit
	trunkCurrent := atomic.LoadInt32(counter)
	if trunkCurrent >= cp.maxPerTrunk {
		log.Printf("[ChannelPool] Trunk '%s' limit reached: %d/%d", trunk, trunkCurrent, cp.maxPerTrunk)
		return false
	}

	// Atomically increment both counters
	// Use CompareAndSwap to prevent race conditions
	for {
		current = atomic.LoadInt32(&cp.activeGlobal)
		if current >= cp.maxGlobal {
			return false
		}
		if atomic.CompareAndSwapInt32(&cp.activeGlobal, current, current+1) {
			break
		}
	}

	for {
		trunkCurrent = atomic.LoadInt32(counter)
		if trunkCurrent >= cp.maxPerTrunk {
			// Rollback global increment
			atomic.AddInt32(&cp.activeGlobal, -1)
			return false
		}
		if atomic.CompareAndSwapInt32(counter, trunkCurrent, trunkCurrent+1) {
			break
		}
	}

	log.Printf("[ChannelPool] Acquired slot: trunk='%s' (global: %d/%d, trunk: %d/%d)",
		trunk,
		atomic.LoadInt32(&cp.activeGlobal), cp.maxGlobal,
		atomic.LoadInt32(counter), cp.maxPerTrunk)

	return true
}

// Release releases a channel slot for the given trunk
func (cp *ChannelPool) Release(trunk string) {
	// Decrement global counter
	newGlobal := atomic.AddInt32(&cp.activeGlobal, -1)
	if newGlobal < 0 {
		// Safety: prevent negative counts
		atomic.StoreInt32(&cp.activeGlobal, 0)
		log.Printf("[ChannelPool] WARNING: Global counter went negative, reset to 0")
	}

	// Decrement per-trunk counter
	if counterI, ok := cp.perTrunk.Load(trunk); ok {
		counter := counterI.(*int32)
		newTrunk := atomic.AddInt32(counter, -1)
		if newTrunk < 0 {
			atomic.StoreInt32(counter, 0)
			log.Printf("[ChannelPool] WARNING: Trunk '%s' counter went negative, reset to 0", trunk)
		}
		log.Printf("[ChannelPool] Released slot: trunk='%s' (global: %d/%d, trunk: %d/%d)",
			trunk,
			atomic.LoadInt32(&cp.activeGlobal), cp.maxGlobal,
			atomic.LoadInt32(counter), cp.maxPerTrunk)
	}
}

// Stats returns current usage statistics
func (cp *ChannelPool) Stats() PoolStats {
	stats := PoolStats{
		MaxGlobal:    int(cp.maxGlobal),
		ActiveGlobal: int(atomic.LoadInt32(&cp.activeGlobal)),
		PerTrunk:     make(map[string]TrunkStats),
	}

	cp.perTrunk.Range(func(key, value interface{}) bool {
		trunk := key.(string)
		counter := value.(*int32)
		stats.PerTrunk[trunk] = TrunkStats{
			Active: int(atomic.LoadInt32(counter)),
			Max:    int(cp.maxPerTrunk),
		}
		return true
	})

	return stats
}

// PoolStats contains pool statistics
type PoolStats struct {
	MaxGlobal    int
	ActiveGlobal int
	PerTrunk     map[string]TrunkStats
}

// TrunkStats contains per-trunk statistics
type TrunkStats struct {
	Active int
	Max    int
}

// Available returns how many slots are available globally
func (cp *ChannelPool) Available() int {
	current := atomic.LoadInt32(&cp.activeGlobal)
	available := int(cp.maxGlobal - current)
	if available < 0 {
		return 0
	}
	return available
}

// AvailableForTrunk returns how many slots are available for a specific trunk
func (cp *ChannelPool) AvailableForTrunk(trunk string) int {
	counterI, ok := cp.perTrunk.Load(trunk)
	if !ok {
		return int(cp.maxPerTrunk)
	}
	counter := counterI.(*int32)
	current := atomic.LoadInt32(counter)
	available := int(cp.maxPerTrunk - current)
	if available < 0 {
		return 0
	}
	return available
}

// SetMaxGlobal updates the global limit dynamically
func (cp *ChannelPool) SetMaxGlobal(max int) {
	atomic.StoreInt32(&cp.maxGlobal, int32(max))
	log.Printf("[ChannelPool] Updated global limit to %d", max)
}

// SetMaxPerTrunk updates the per-trunk limit dynamically
func (cp *ChannelPool) SetMaxPerTrunk(max int) {
	atomic.StoreInt32(&cp.maxPerTrunk, int32(max))
	log.Printf("[ChannelPool] Updated per-trunk limit to %d", max)
}
