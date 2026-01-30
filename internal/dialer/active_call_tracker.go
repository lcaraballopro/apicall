package dialer

import (
	"log"
	"sync"
	"time"
)

// ActiveCall represents an in-progress call
type ActiveCall struct {
	UniqueID   string
	LogID      int64
	ContactID  int64
	CampaignID int
	ProyectoID int
	Trunk      string
	Telefono   string
	StartTime  time.Time
}

// ActiveCallTracker tracks all active calls for correlation and cleanup
type ActiveCallTracker struct {
	calls   map[string]*ActiveCall // uniqueID (Internal UUID) -> ActiveCall
	aliases map[string]string      // asteriskID -> uniqueID (Internal UUID)
	mu      sync.RWMutex
}

// NewActiveCallTracker creates a new tracker
func NewActiveCallTracker() *ActiveCallTracker {
	return &ActiveCallTracker{
		calls:   make(map[string]*ActiveCall),
		aliases: make(map[string]string),
	}
}

// Add registers a new active call
func (t *ActiveCallTracker) Add(call *ActiveCall) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls[call.UniqueID] = call
	log.Printf("[ActiveCallTracker] Added call %s (contact=%d, campaign=%d)", 
		call.UniqueID, call.ContactID, call.CampaignID)
}

// Get retrieves an active call by uniqueID
func (t *ActiveCallTracker) Get(uniqueID string) *ActiveCall {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.calls[uniqueID]
}

// Remove removes a call from tracking
func (t *ActiveCallTracker) Remove(uniqueID string) *ActiveCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	call, ok := t.calls[uniqueID]
	if ok {
		delete(t.calls, uniqueID)
		
		// Remove any alias pointing to this call
		// This is O(N) unfortunately, but N (aliases) is small per call (0 or 1)
		// Better approach: store reverse alias in ActiveCall provided we update struct
		for k, v := range t.aliases {
			if v == uniqueID {
				delete(t.aliases, k)
			}
		}
		
		log.Printf("[ActiveCallTracker] Removed call %s (duration: %v)", 
			uniqueID, time.Since(call.StartTime))
	}
	return call
}

// Count returns the number of active calls
func (t *ActiveCallTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.calls)
}

// GetStale returns calls older than the specified duration
// This is used by the orphan cleaner to find stuck calls
func (t *ActiveCallTracker) GetStale(maxAge time.Duration) []*ActiveCall {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	var stale []*ActiveCall
	threshold := time.Now().Add(-maxAge)
	
	for _, call := range t.calls {
		if call.StartTime.Before(threshold) {
			stale = append(stale, call)
		}
	}
	
	return stale
}

// CountByTrunk returns call counts grouped by trunk
func (t *ActiveCallTracker) CountByTrunk() map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	counts := make(map[string]int)
	for _, call := range t.calls {
		counts[call.Trunk]++
	}
	return counts
}

// CountByCampaign returns call counts grouped by campaign
func (t *ActiveCallTracker) CountByCampaign() map[int]int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	counts := make(map[int]int)
	for _, call := range t.calls {
		if call.CampaignID > 0 {
			counts[call.CampaignID]++
		}
	}
	return counts
}

// List returns all active calls (for debugging/monitoring)
func (t *ActiveCallTracker) List() []*ActiveCall {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	calls := make([]*ActiveCall, 0, len(t.calls))
	for _, call := range t.calls {
		calls = append(calls, call)
	}
	return calls
}

// AddAlias adds an alias (e.g. Asterisk ID) for an existing call
func (t *ActiveCallTracker) AddAlias(alias, uniqueID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Verify call exists
	if _, ok := t.calls[uniqueID]; ok {
		t.aliases[alias] = uniqueID
		log.Printf("[ActiveCallTracker] Linked alias %s -> %s", alias, uniqueID)
	}
}

// GetByAlias retrieves a call by its alias (e.g. Asterisk ID)
func (t *ActiveCallTracker) GetByAlias(alias string) *ActiveCall {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if uniqueID, ok := t.aliases[alias]; ok {
		return t.calls[uniqueID]
	}
	return nil
}
