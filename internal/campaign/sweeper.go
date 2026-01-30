package campaign

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"apicall/internal/database"
	"apicall/internal/dialer"
)

const (
	// SweeperInterval is how often the sweeper checks for work
	SweeperInterval = 1 * time.Second
	// DefaultContactsPerCycle is the default if not configured in DB
	DefaultContactsPerCycle = 100
)

// Sweeper processes active campaigns
type Sweeper struct {
	repo      *database.Repository
	dialer    *dialer.AMIDialer
	running   bool
	stopChan  chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
}

// NewSweeper creates a new campaign sweeper
func NewSweeper(repo *database.Repository, d *dialer.AMIDialer) *Sweeper {
	return &Sweeper{
		repo:     repo,
		dialer:   d,
		stopChan: make(chan struct{}),
	}
}

// Start begins the sweeper worker
func (s *Sweeper) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.wg.Add(1)
	s.mu.Unlock()

	go s.run()
	log.Println("[Sweeper] Campaign sweeper started")
}

// Stop gracefully stops the sweeper
func (s *Sweeper) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()
	log.Println("[Sweeper] Campaign sweeper stopped")
}

func (s *Sweeper) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(SweeperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.processCampaigns()
		}
	}
}

func (s *Sweeper) processCampaigns() {
	// Get all active campaigns
	campaigns, err := s.repo.GetActiveCampaigns()
	if err != nil {
		log.Printf("[Sweeper] Error fetching active campaigns: %v", err)
		return
	}

	if len(campaigns) == 0 {
		return // Nothing to process
	}

	for _, campaign := range campaigns {
		s.processCampaign(&campaign)
	}
}

func (s *Sweeper) processCampaign(campaign *database.Campaign) {
	// Check if within schedule
	inSchedule, err := s.repo.IsWithinSchedule(campaign.ID)
	if err != nil {
		log.Printf("[Sweeper] Error checking schedule for campaign %d: %v", campaign.ID, err)
		return
	}

	if !inSchedule {
		// Not within schedule, skip
		return
	}

	// Get pending contacts (read config dynamically from DB)
	contactsPerCycle := s.getContactsPerCycle()
	contacts, err := s.repo.GetPendingContacts(campaign.ID, contactsPerCycle)
	if err != nil {
		log.Printf("[Sweeper] Error fetching contacts for campaign %d: %v", campaign.ID, err)
		return
	}

	if len(contacts) == 0 {
		// Check if campaign is complete
		counts, _ := s.repo.CountContactsByStatus(campaign.ID)
		pending := counts["pending"]
		dialing := counts["dialing"]
		
		if pending == 0 && dialing == 0 {
			// All contacts processed, mark campaign as completed
			log.Printf("[Sweeper] Campaign %d completed - all contacts processed", campaign.ID)
			s.repo.UpdateCampaignStatus(campaign.ID, "completed")
		}
		return
	}

	// Get the project for this campaign
	proyecto, err := s.repo.GetProyecto(campaign.ProyectoID)
	if err != nil {
		log.Printf("[Sweeper] Error fetching project %d for campaign %d: %v", 
			campaign.ProyectoID, campaign.ID, err)
		return
	}

	// Process contacts
	for _, contact := range contacts {
		// Check blacklist
		blacklisted, _ := s.repo.IsBlacklisted(campaign.ProyectoID, contact.Telefono)
		if blacklisted {
			log.Printf("[Sweeper] Skipping blacklisted number %s in campaign %d", contact.Telefono, campaign.ID)
			skipped := "BLACKLISTED"
			s.repo.UpdateContactStatus(contact.ID, "skipped", &skipped)
			continue
		}

		// Mark as dialing
		s.repo.MarkContactDialing(contact.ID)

		// Execute dial in goroutine to not block sweeper
		go func(c database.CampaignContact, p *database.Proyecto, campID int) {
			req := dialer.DialRequest{
				CampaignID:  campID,
				ContactID:   c.ID,
				Project:     p,
				Destination: c.Telefono,
				Variables:   make(map[string]string),
				Timeout:     45 * time.Second, // Standard dial timeout
			}

			if err := s.dialer.Dial(req); err != nil {
				// Failed to initiate
				log.Printf("[Sweeper] Dial failed for %s: %v", c.Telefono, err)
				
				// Check error type to decide if we should retry or fail
				errMsg := err.Error()
				var newStatus string = "pending"
				var reason string = "RETRY"
				
				if strings.Contains(errMsg, "reason: 5") { // Busy
					newStatus = "failed"
					reason = "BUSY"
				} else if strings.Contains(errMsg, "reason: 8") { // Congestion
					newStatus = "failed"
					reason = "CONGESTION"
				} else if strings.Contains(errMsg, "reason: 1") { // Invalid
					newStatus = "failed"
					reason = "INVALID"
				} else if strings.Contains(errMsg, "channel limit") {
					// Pool full, keep pending for retry
					newStatus = "pending"
					reason = "LIMIT"
				}
				
				// Update status
				var reasonPtr *string
				if reason != "RETRY" {
					reasonPtr = &reason
				}
				s.repo.UpdateContactStatus(c.ID, newStatus, reasonPtr)

			} else {
				log.Printf("[Sweeper] Call initiated for campaign %d: %s (contact_id=%d)", campID, c.Telefono, c.ID)
			}
		}(contact, proyecto, campaign.ID)
	}

	// Update campaign stats (roughly)
	counts, _ := s.repo.CountContactsByStatus(campaign.ID)
	processed := counts["completed"] + counts["failed"] + counts["skipped"]
	s.repo.UpdateCampaignStats(campaign.ID, processed, counts["completed"], counts["failed"])
}

// getContactsPerCycle reads the contacts_per_cycle config from database
// This allows dynamic configuration changes without service restart
func (s *Sweeper) getContactsPerCycle() int {
	val, err := s.repo.GetConfig("contacts_per_cycle")
	if err != nil || val == "" {
		return DefaultContactsPerCycle
	}
	
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return DefaultContactsPerCycle
	}
	
	return n
}
