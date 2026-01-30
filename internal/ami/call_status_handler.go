package ami

import (
	"log"
	"strconv"
	"strings"

	"apicall/internal/database"
)

// CallTracker defines the interface for tracking and releasing calls
type CallTracker interface {
	GetContactID(uniqueID string) (int64, bool)
	Release(uniqueID string)
	AddAlias(alias, uniqueID string)
}

// CallStatusHandler processes AMI events to update call statuses
type CallStatusHandler struct {
	client  *Client
	repo    *database.Repository
	tracker CallTracker
	done    chan struct{}
}

// NewCallStatusHandler creates a new handler
func NewCallStatusHandler(client *Client, repo *database.Repository, tracker CallTracker) *CallStatusHandler {
	return &CallStatusHandler{
		client:  client,
		repo:    repo,
		tracker: tracker,
		done:    make(chan struct{}),
	}
}

// Start begins processing AMI events
func (h *CallStatusHandler) Start() {
	go h.processEvents()
	log.Println("[AMI-Handler] Call status handler started")
}

// Stop stops the handler
func (h *CallStatusHandler) Stop() {
	close(h.done)
	log.Println("[AMI-Handler] Call status handler stopped")
}

func (h *CallStatusHandler) processEvents() {
	for {
		select {
		case <-h.done:
			return
		case event, ok := <-h.client.Events():
			if !ok {
				return
			}
			h.handleEvent(event)
		}
	}
}

func (h *CallStatusHandler) handleEvent(event Event) {
	// We're interested in Hangup events for calls that never reached AGI
	// and OriginateResponse for failed originations
	
	switch event.Type {
	case "Hangup":
		h.handleHangup(event)
	case "OriginateResponse":
		h.handleOriginateResponse(event)
	case "VarSet":
		h.handleVarSet(event)
	}
}

// handleHangup processes Hangup events to update call status
func (h *CallStatusHandler) handleHangup(event Event) {
	// Get the APICALL_LOG_ID from channel variables
	// This is set in the .call file
	channel := event.Fields["Channel"]
	cause := event.Fields["Cause"]
	causeText := event.Fields["Cause-txt"]
	
	// Only process SIP channels (our outbound calls)
	if !strings.HasPrefix(channel, "SIP/") {
		return
	}
	
	// Try to get our log ID from the Uniqueid
	uniqueid := event.Fields["Uniqueid"]
	if uniqueid == "" {
		return
	}
	
	// Map Asterisk cause codes to standard Contact Center dispositions
	// See: https://wiki.asterisk.org/wiki/display/AST/Hangup+Cause+Mappings
	// Standard codes: A=Answered, AM=AnsweringMachine, B=Busy, NA=NoAnswer,
	//                 NI=InvalidNumber, CONG=Congestion, FAIL=Failed, XFER=Transferred
	var status string
	var disposition string
	
	causeInt, _ := strconv.Atoi(cause)
	switch causeInt {
	case 16: // Normal clearing
		// This is normal hangup, AGI should have handled it
		// Only update if still DIALING (missed by AGI somehow)
		status = "COMPLETED"
		disposition = "A" // Answered/Contacted
	case 17: // User busy
		status = "COMPLETED"
		disposition = "B" // Busy
	case 18, 19: // No user responding, No answer
		status = "COMPLETED"
		disposition = "NA" // No Answer
	case 21: // Call rejected
		status = "COMPLETED"
		disposition = "NA" // No Answer (rejected)
	case 27: // Destination out of order
		status = "FAILED"
		disposition = "NI" // Invalid Number
	case 34, 38: // No circuit/network congestion
		status = "FAILED"
		disposition = "CONG" // Congestion
	case 1: // Unallocated number
		status = "FAILED"
		disposition = "NI" // Invalid Number
	default:
		// Unknown cause, mark as no answer
		status = "COMPLETED"
		disposition = "NA" // No Answer
	}
	
	// Find and update any DIALING call with this uniqueid
	// We need to search by uniqueid pattern (the .call file includes it in channel name)
	updated, err := h.repo.UpdateDialingCallByUniqueid(uniqueid, status, disposition)
	if err != nil {
		log.Printf("[AMI-Handler] Error updating call: %v", err)
		return
	}
	
	// Release channel slot and update contact if this was a tracked call
	if h.tracker != nil {
		contactID, exists := h.tracker.GetContactID(uniqueid)
		if exists {
			h.tracker.Release(uniqueid)
			// Update contact status
			if contactID > 0 {
				contactStatus := "failed"
				if disposition == "A" || disposition == "XFER" {
					contactStatus = "completed"
				}
				h.repo.UpdateContactStatus(contactID, contactStatus, &status)
				log.Printf("[AMI-Handler] Updated contact %d -> %s", contactID, contactStatus)
			}
		}
	}
	
	if updated {
		log.Printf("[AMI-Handler] Updated call %s: %s (%s)", uniqueid, status, causeText)
	}
}

// handleOriginateResponse processes failed originations
func (h *CallStatusHandler) handleOriginateResponse(event Event) {
	response := event.Fields["Response"]
	if response == "Success" {
		return // Call was answered, AGI will handle it
	}
	
	// Failed origination - map to standard Contact Center dispositions
	reason := event.Fields["Reason"]
	uniqueid := event.Fields["Uniqueid"]
	
	var status string
	var disposition string
	switch reason {
	case "0": // No reason
		status = "FAILED"
		disposition = "FAIL"
	case "1": // No such channel
		status = "FAILED"
		disposition = "NI" // Invalid Number
	case "4": // Answer
		return // AGI handles this
	case "5": // Busy
		status = "COMPLETED"
		disposition = "B" // Busy
	case "8": // Congestion
		status = "FAILED"
		disposition = "CONG"
	default:
		status = "FAILED"
		disposition = "FAIL"
	}
	
	if uniqueid != "" {
		updated, _ := h.repo.UpdateDialingCallByUniqueid(uniqueid, status, disposition)
		if updated {
			log.Printf("[AMI-Handler] Originate failed %s: %s (disposition: %s)", uniqueid, status, disposition)
		}
		// Note: We do NOT release the tracker here.
		// AMIDialer handles the release on failure (synchronously).
		// CallStatusHandler only releases on Hangup for established calls.
	}
}

// handleVarSet processes variable updates to link Asterisk ID with our UniqueID
func (h *CallStatusHandler) handleVarSet(event Event) {
	// We are listening for APICALL_UNIQUEID being set on the channel
	variable := event.Fields["Variable"]
	if variable != "APICALL_UNIQUEID" {
		return
	}
	
	// Asterisk UniqueID (The Alias)
	asteriskID := event.Fields["Uniqueid"]
	// Our Internal UUID (The Value)
	internalUUID := event.Fields["Value"]
	
	if asteriskID != "" && internalUUID != "" && h.tracker != nil {
		log.Printf("[AMI-Handler] DEBUG: VarSet detected. Linking AsteriskID=%s -> UUID=%s", asteriskID, internalUUID)
		h.tracker.AddAlias(asteriskID, internalUUID)
	}
}
