package dialer

import (
	"log"
)

// CallManager coordinates between ChannelPool and ActiveCallTracker
// and satisfies the ami.CallTracker interface
type CallManager struct {
	pool    *ChannelPool
	tracker *ActiveCallTracker
}

func NewCallManager(pool *ChannelPool, tracker *ActiveCallTracker) *CallManager {
	return &CallManager{
		pool:    pool,
		tracker: tracker,
	}
}

// GetContactID retrieves the contact ID for a given uniqueID
func (m *CallManager) GetContactID(uniqueID string) (int64, bool) {
	// Try direct
	call := m.tracker.Get(uniqueID)
	if call == nil {
		// Try alias
		call = m.tracker.GetByAlias(uniqueID)
	}

	if call != nil {
		return call.ContactID, true
	}
	return 0, false
}

// AddAlias links an alias (e.g. Asterisk ID) to an internal uniqueID
func (m *CallManager) AddAlias(alias, uniqueID string) {
	m.tracker.AddAlias(alias, uniqueID)
}

// Release releases the channel slot and removes tracking
func (m *CallManager) Release(uniqueID string) {
	// Resolve if alias
	targetID := uniqueID
	call := m.tracker.GetByAlias(uniqueID)
	if call != nil {
		targetID = call.UniqueID
	} else {
		// Check if it's a direct ID
		call = m.tracker.Get(uniqueID)
	}

	// Remove from tracker
	// Remove returns the call object, so we can get Trunk info if we didn't have it
	removedCall := m.tracker.Remove(targetID)
	
	if removedCall != nil {
		// Release slot based on Trunk
		m.pool.Release(removedCall.Trunk)
		log.Printf("[CallManager] Released call %s (trunk=%s)", targetID, removedCall.Trunk)
	} else {
		// If we couldn't find it in tracker, we might still need to release if we knew the trunk
		// But without tracker we don't know which trunk it used.
		// However, ChannelPool Release takes a trunk name.
		// If we don't have the call, we can't release the specific trunk slot.
		// This implies we rely entirely on Tracker to know what to release.
		log.Printf("[CallManager] WARNING: Requested release for unknown call %s", uniqueID)
	}
}
