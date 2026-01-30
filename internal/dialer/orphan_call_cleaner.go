package dialer

import (
	"log"
	"sync"
	"time"

	"apicall/internal/database"
)

// OrphanCallCleaner periodically cleans up orphaned calls and contacts
// This handles cases where:
// - Calls stuck in DIALING status for too long
// - Contacts stuck in "dialing" state
// - Channel slots that weren't properly released
type OrphanCallCleaner struct {
	repo        *database.Repository
	channelPool *ChannelPool
	callTracker *ActiveCallTracker
	
	interval    time.Duration
	maxCallAge  time.Duration
	
	running     bool
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
}

// NewOrphanCallCleaner creates a new cleaner
func NewOrphanCallCleaner(repo *database.Repository, pool *ChannelPool, tracker *ActiveCallTracker) *OrphanCallCleaner {
	return &OrphanCallCleaner{
		repo:        repo,
		channelPool: pool,
		callTracker: tracker,
		interval:    10 * time.Second,
		maxCallAge:  60 * time.Second,
		stopChan:    make(chan struct{}),
	}
}

// Start begins the orphan cleaner worker
func (c *OrphanCallCleaner) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.wg.Add(1)
	c.mu.Unlock()

	go c.run()
	log.Println("[OrphanCleaner] Started")
}

// Stop stops the cleaner
func (c *OrphanCallCleaner) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopChan)
	c.wg.Wait()
	log.Println("[OrphanCleaner] Stopped")
}

func (c *OrphanCallCleaner) run() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Run once immediately
	c.cleanup()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

func (c *OrphanCallCleaner) cleanup() {
	// 1. Clean up stale tracked calls
	c.cleanupStaleCalls()
	
	// 2. Clean up orphaned DB records
	c.cleanupOrphanedCallLogs()
	
	// 3. Clean up orphaned contacts
	c.cleanupOrphanedContacts()
}

// cleanupStaleCalls removes calls from tracker that are too old
func (c *OrphanCallCleaner) cleanupStaleCalls() {
	if c.callTracker == nil {
		return
	}

	staleCalls := c.callTracker.GetStale(c.maxCallAge)
	for _, call := range staleCalls {
		// Remove from tracker
		c.callTracker.Remove(call.UniqueID)
		
		// Release channel slot
		if c.channelPool != nil {
			c.channelPool.Release(call.Trunk)
		}
		
		// Update call log to COMPLETED with NA (no answer) disposition
		if call.LogID > 0 {
			na := "NA" // Standard: No Answer
			c.repo.UpdateCallLog(call.LogID, nil, &na, nil, false, "COMPLETED", 0)
		}
		
		// Update contact to failed if applicable
		if call.ContactID > 0 {
			na := "NA" // Standard: No Answer
			c.repo.UpdateContactStatus(call.ContactID, "failed", &na)
		}
		
		log.Printf("[OrphanCleaner] Cleaned stale call: uniqueID=%s, age=%v", 
			call.UniqueID, time.Since(call.StartTime))
	}
	
	if len(staleCalls) > 0 {
		log.Printf("[OrphanCleaner] Cleaned %d stale calls from tracker", len(staleCalls))
	}
}

// cleanupOrphanedCallLogs finds and updates call logs stuck in DIALING
func (c *OrphanCallCleaner) cleanupOrphanedCallLogs() {
	if c.repo == nil {
		return
	}

	// Find calls stuck in DIALING for more than 5 minutes
	// Using standard codes: COMPLETED + NA (no answer)
	query := `
		UPDATE apicall_call_log 
		SET status = 'COMPLETED', disposition = 'NA'
		WHERE status = 'DIALING' 
		  AND created_at < NOW() - INTERVAL 5 MINUTE
	`
	result, err := c.repo.GetDB().Exec(query)
	if err != nil {
		log.Printf("[OrphanCleaner] Error cleaning orphaned call logs: %v", err)
		return
	}
	
	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[OrphanCleaner] Cleaned %d orphaned call logs (DIALING > 5min)", rows)
	}
}

// cleanupOrphanedContacts finds and updates contacts stuck in dialing state
func (c *OrphanCallCleaner) cleanupOrphanedContacts() {
	if c.repo == nil {
		return
	}

	// Find contacts stuck in "dialing" for more than 5 minutes
	// Using standard code: NA (no answer)
	query := `
		UPDATE apicall_campaign_contacts 
		SET estado = 'failed', resultado = 'NA'
		WHERE estado = 'dialing' 
		  AND ultimo_intento IS NOT NULL
		  AND ultimo_intento < NOW() - INTERVAL 5 MINUTE
	`
	result, err := c.repo.GetDB().Exec(query)
	if err != nil {
		log.Printf("[OrphanCleaner] Error cleaning orphaned contacts: %v", err)
		return
	}
	
	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[OrphanCleaner] Cleaned %d orphaned contacts (dialing > 5min)", rows)
	}
}

// SetInterval configures the cleanup interval
func (c *OrphanCallCleaner) SetInterval(interval time.Duration) {
	c.interval = interval
}

// SetMaxCallAge configures the max age for calls before they're considered orphaned
func (c *OrphanCallCleaner) SetMaxCallAge(maxAge time.Duration) {
	c.maxCallAge = maxAge
}
