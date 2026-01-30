package dialer

import (
	"fmt"
	"log"
	"sync"
	"time"

	"apicall/internal/ami"
	"apicall/internal/database"
	"apicall/internal/smartcid"
)

// DialRequest contains the specific details for a single call
type DialRequest struct {
	CampaignID    int
	ContactID     int64
	Project       *database.Proyecto
	Destination   string
	Variables     map[string]string
	Timeout       time.Duration
}

// AMIDialer handles synchronous dialing via AMI
type AMIDialer struct {
	client      *ami.Client
	pool        *ChannelPool
	tracker     *ActiveCallTracker
	repo        *database.Repository
	scidGen     *smartcid.Generator

	// Event Dispatching
	mu          sync.RWMutex
	pending     map[string]chan ami.Event
	stopChan    chan struct{}
	running     bool
}

// NewAMIDialer creates a new dialer
func NewAMIDialer(client *ami.Client, pool *ChannelPool, tracker *ActiveCallTracker, repo *database.Repository) *AMIDialer {
	return &AMIDialer{
		client:   client,
		pool:     pool,
		tracker:  tracker,
		repo:     repo,
		pending:  make(map[string]chan ami.Event),
		stopChan: make(chan struct{}),
	}
}

// SetSmartCIDGenerator sets the Smart Caller ID generator
func (d *AMIDialer) SetSmartCIDGenerator(gen *smartcid.Generator) {
	d.scidGen = gen
	log.Printf("[AMIDialer] Smart CID Generator configured")
}

// Start begins the event listener loop
func (d *AMIDialer) Start() {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.mu.Unlock()

	go d.listenEvents()
	log.Println("[AMIDialer] Started event listener")
}

// Stop stops the dialer
func (d *AMIDialer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	d.running = false
	close(d.stopChan)
}

func (d *AMIDialer) listenEvents() {
	// Single persistent subscription
	events := d.client.Subscribe()

	for {
		select {
		case <-d.stopChan:
			return
		case event := <-events:
			if event.Type == "OriginateResponse" {
				actionID := event.Fields["ActionID"]
				if actionID != "" {
					d.dispatch(actionID, event)
				}
			}
		}
	}
}

func (d *AMIDialer) dispatch(actionID string, event ami.Event) {
	d.mu.RLock()
	ch, exists := d.pending[actionID]
	d.mu.RUnlock()

	if exists {
		// Non-blocking send just in case
		select {
		case ch <- event:
		default:
		}
	}
}

// Dial executes a call synchronously using AMI Originate
func (d *AMIDialer) Dial(req DialRequest) error {
	// 1. Acquire Channel Slot
	if !d.pool.Acquire(req.Project.TroncalSalida) {
		return fmt.Errorf("channel limit reached for trunk %s", req.Project.TroncalSalida)
	}

	// Track if we need to release slot (set to false on successful answer/handover)
	// Actually, tracker logic: Handover happens via VarSet/Hangup. 
	// If Dial returns Success, the call IS active in Asterisk, so Tracker takes over.
	// If Dial returns Fail, the call is DEAD, so WE must release.
	releaseRequired := true
	defer func() {
		if releaseRequired {
			d.pool.Release(req.Project.TroncalSalida)
			// Also remove from tracker if it was added
		}
	}()

	// 2. Setup ID and Tracking
	internalUUID := fmt.Sprintf("%d-%d-%d", req.CampaignID, req.ContactID, time.Now().UnixNano())
	actionID := "act-" + internalUUID

	// 3. Smart Caller ID Determination
	callerID := req.Project.CallerID
	if d.scidGen != nil && req.Project.SmartCIDActive {
		generatedCID := d.scidGen.GetCallerID(req.Destination, callerID, req.Project.SmartCIDActive)
		log.Printf("[AMIDialer] Smart CID: Proyecto=%d, Destino=%s, Original=%s, Generado=%s",
			req.Project.ID, req.Destination, callerID, generatedCID)
		callerID = generatedCID
	} else {
		log.Printf("[AMIDialer] Using static CID: Proyecto=%d, CID=%s (SmartGen=%v, SmartActive=%v)",
			req.Project.ID, callerID, d.scidGen != nil, req.Project.SmartCIDActive)
	}

	// 4. Create CallLog in database for tracking
	var campaignID *int
	if req.CampaignID > 0 {
		cid := req.CampaignID
		campaignID = &cid
	}

	callLog := &database.CallLog{
		ProyectoID:   req.Project.ID,
		Telefono:     req.Destination,
		Status:       "DIALING",
		Interacciono: false,
		CallerIDUsed: callerID,
		CampaignID:   campaignID,
	}

	logID, err := d.repo.CreateCallLog(callLog)
	if err != nil {
		log.Printf("[AMIDialer] Error creating call log: %v", err)
		// Continue anyway, don't fail the call just because logging failed
	} else {
		log.Printf("[AMIDialer] Created call log ID=%d for campaign=%d contact=%d callerID=%s", logID, req.CampaignID, req.ContactID, callerID)
	}

	// Register in Tracker (Pending) - include LogID for later updates
	call := &ActiveCall{
		UniqueID:   internalUUID,
		Trunk:      req.Project.TroncalSalida,
		StartTime:  time.Now(),
		CampaignID: req.CampaignID,
		ContactID:  req.ContactID,
		ProyectoID: req.Project.ID,
		LogID:      logID,
	}
	d.tracker.Add(call)

	defer func() {
		if releaseRequired {
			d.tracker.Remove(internalUUID)
		}
	}()

	// 3. Prepare result channel
	respChan := make(chan ami.Event, 1)
	d.mu.Lock()
	d.pending[actionID] = respChan
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pending, actionID)
		d.mu.Unlock()
	}()

	// 4. Construct AMI Action
	// Build channel string: SIP/trunk/prefix+number
	// Assuming logic from spooler for prefix construction:
	// "SIP/%s/%s%s", proyecto.TroncalSalida, proyecto.PrefijoSalida, telefono
	// We need 'dest' passed fully formed or constructed here. 
	// The Req has Destination. Assuming it's just the number.
	// Let's assume Caller ensures full number format or we do it here.
	// Sweeper logic adds prefix. Let's assume Req has full dial string or parts.
	// Based on sweeper.go, it passes 'telefono'. 
	// Standard: Local/number@context or SIP/trunk/number.
	// Let's use Local channel for flexibility or direct endpoint if configured.
	// Spooler uses: fmt.Sprintf("SIP/%s/%s%s", proyecto.TroncalSalida, proyecto.PrefijoSalida, telefono)
	
	dialString := fmt.Sprintf("SIP/%s/%s%s", req.Project.TroncalSalida, req.Project.PrefijoSalida, req.Destination)
	
	vars := ""
	for k, v := range req.Variables {
		if vars != "" {
			vars += ","
		}
		vars += fmt.Sprintf("%s=%s", k, v)
	}
	// Add critical tracking vars
	if vars != "" { vars += "," }
	vars += fmt.Sprintf("APICALL_UNIQUEID=%s", internalUUID)
	vars += fmt.Sprintf(",APICALL_PROJECT_ID=%d", req.Project.ID)
	vars += fmt.Sprintf(",APICALL_CAMPAIGN_ID=%d", req.CampaignID)
	vars += fmt.Sprintf(",APICALL_CONTACT_ID=%d", req.ContactID)
	// CRITICAL: Pass the LogID so AGI knows which log to update!
	vars += fmt.Sprintf(",APICALL_LOG_ID=%d", logID)

	action := fmt.Sprintf(
		"Action: Originate\r\n"+
		"ActionID: %s\r\n"+
		"Channel: %s\r\n"+
		"Context: %s\r\n"+
		"Exten: s\r\n"+
		"Priority: 1\r\n"+
		"CallerID: %s\r\n"+
		"Timeout: %d\r\n"+
		"Async: true\r\n"+
		"Variable: %s\r\n"+
		"\r\n",
		actionID,
		dialString,
		"apicall_context", // Hardcoded context matching dialplan
		callerID, // Smart CID if active, otherwise project CallerID
		int(req.Timeout.Milliseconds()),
		vars,
	)

	// 5. Send Action
	if err := d.client.SendAction(action); err != nil {
		return fmt.Errorf("failed to send originate: %w", err)
	}

	// 6. Wait for Response
	select {
	case event := <-respChan:
		response := event.Fields["Response"]
		if response == "Success" {
			// Call Initiated Successfully!
			// Tracker and AMI Handler will take over monitoring lifecycle.
			releaseRequired = false // Do NOT release slot/tracker here
			return nil
		}
		// Failure (Busy, Congestion, etc handled by OriginateResponse Reason usually, but if 'Response' is fail...)
		reason := event.Fields["Reason"] // 0=Fail, 1=NoExist, 3=RingTimeout, 5=Busy, 8=Congestion
		return fmt.Errorf("originate failed: %s (reason: %s)", response, reason)

	case <-time.After(req.Timeout + 5*time.Second):
		// Use a buffer over expected timeout
		return fmt.Errorf("originate timeout mismatch (no response from AMI)")
	}
}
