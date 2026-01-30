package database

import (
	"log"
	"sync"
	"time"
)

const (
	// OrphanCleanerInterval is how often to check for orphaned calls
	OrphanCleanerInterval = 30 * time.Second
	// OrphanThreshold is how long a call can stay in DIALING before being marked orphaned
	OrphanThreshold = 2 * time.Minute
)

// OrphanCallCleaner periodically cleans up calls stuck in DIALING status
type OrphanCallCleaner struct {
	repo     *Repository
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewOrphanCallCleaner creates a new cleaner
func NewOrphanCallCleaner(repo *Repository) *OrphanCallCleaner {
	return &OrphanCallCleaner{
		repo:     repo,
		stopChan: make(chan struct{}),
	}
}

// Start begins the cleaner worker
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
	log.Println("[OrphanCleaner] Started - checking every 30s for calls stuck in DIALING > 2min")
}

// Stop gracefully stops the cleaner
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

	ticker := time.NewTicker(OrphanCleanerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.cleanOrphanedCalls()
		}
	}
}

func (c *OrphanCallCleaner) cleanOrphanedCalls() {
	// Update calls that have been in DIALING for more than 2 minutes
	// Using standard Contact Center codes: N=No Interest/Timeout, NA=No Answer
	query := `
		UPDATE apicall_call_log 
		SET status = 'COMPLETED', disposition = 'NA'
		WHERE status = 'DIALING' 
		  AND created_at < NOW() - INTERVAL 2 MINUTE
	`
	
	result, err := c.repo.conn.DB.Exec(query)
	if err != nil {
		log.Printf("[OrphanCleaner] Error cleaning orphaned calls: %v", err)
		return
	}
	
	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[OrphanCleaner] Cleaned %d orphaned calls (DIALING > 2min -> TIMEOUT)", rows)
		
		// Also sync campaign contacts
		c.syncCampaignContacts()
	}
}

func (c *OrphanCallCleaner) syncCampaignContacts() {
	// Sync campaign contacts with the updated logs
	query := `
		UPDATE apicall_campaign_contacts cc
		INNER JOIN apicall_call_log cl ON cc.telefono = cl.telefono
		INNER JOIN apicall_campaigns camp ON cc.campaign_id = camp.id AND camp.proyecto_id = cl.proyecto_id
		SET 
			cc.estado = CASE 
				WHEN cl.status IN ('ANSWERED', 'ANSWER', 'AMD_HUMAN', 'COMPLETED', 'A') THEN 'completed'
				WHEN cl.status IN ('NOANSWER', 'NO ANSWER', 'BUSY', 'FAILED', 'CONGESTION', 'CANCEL', 'TIMEOUT', 'AMD_MACHINE', 'NA', 'B', 'N', 'AM', 'FAIL', 'CONG') THEN 'failed'
				WHEN cl.status IN ('BLACKLISTED', 'DNC') THEN 'skipped'
				WHEN cl.status IN ('XFER', 'TRANSFERRED') THEN 'completed'
				ELSE 'failed'
			END,
			cc.resultado = cl.status,
			cc.ultimo_intento = NOW()
		WHERE cc.estado = 'dialing'
		  AND cl.status != 'DIALING'
		  AND cl.created_at > NOW() - INTERVAL 1 DAY
	`
	
	result, err := c.repo.conn.DB.Exec(query)
	if err != nil {
		log.Printf("[OrphanCleaner] Error syncing campaign contacts: %v", err)
		return
	}
	
	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[OrphanCleaner] Synced %d campaign contacts", rows)
	}
}
