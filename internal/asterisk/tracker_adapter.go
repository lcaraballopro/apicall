package asterisk

// SpoolerTracker implements the ami.CallTracker interface
type SpoolerTracker struct{}

// NewSpoolerTracker creates a new tracker instance
func NewSpoolerTracker() *SpoolerTracker {
	return &SpoolerTracker{}
}

// GetContactID retrieves the contact ID for a given uniqueID
func (t *SpoolerTracker) GetContactID(uniqueID string) (int64, bool) {
	// Try direct lookup first
	call := GetActiveCall(uniqueID)
	
	// If not found, try by alias (Asterisk ID -> Internal UUID)
	if call == nil && callTracker != nil {
		call = callTracker.GetByAlias(uniqueID)
	}

	if call != nil {
		return call.ContactID, true
	}
	return 0, false
}

// AddAlias adds an alias (e.g. Asterisk ID) for an existing call
func (t *SpoolerTracker) AddAlias(alias, uniqueID string) {
	if callTracker != nil {
		callTracker.AddAlias(alias, uniqueID)
	}
}

// Release releases the channel slot for a given uniqueID
func (t *SpoolerTracker) Release(uniqueID string) {
	// If uniqueID is an alias (Asterisk ID), resolve it first
	if callTracker != nil {
		if call := callTracker.GetByAlias(uniqueID); call != nil {
			ReleaseChannel(call.UniqueID)
			return
		}
	}
	ReleaseChannel(uniqueID)
}
