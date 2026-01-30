package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	BatchSize     = 1000
	FlushInterval = 500 * time.Millisecond
	BufferSize    = 5000
)

// LogUpdate represents a pending update to a call log
type LogUpdate struct {
	ID           int64
	DTMFMarcado  *string
	Disposition  *string
	Uniqueid     *string
	Interacciono bool
	Status       string
	Duracion     int
}

// LogBatcher manages buffered updates
type LogBatcher struct {
	db        *sql.DB
	updates   chan LogUpdate
	done      chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	isRunning bool
}

// NewLogBatcher creates a new batcher
func NewLogBatcher(db *sql.DB) *LogBatcher {
	return &LogBatcher{
		db:      db,
		updates: make(chan LogUpdate, BufferSize),
		done:    make(chan struct{}),
	}
}

// Start initiates the background worker
func (b *LogBatcher) Start() {
	b.mu.Lock()
	if b.isRunning {
		b.mu.Unlock()
		return
	}
	b.isRunning = true
	b.wg.Add(1)
	b.mu.Unlock()

	go b.worker()
	log.Println("[LogBatcher] Worker started")
}

// Stop flushes remaining items and stops the worker
func (b *LogBatcher) Stop() {
	b.mu.Lock()
	if !b.isRunning {
		b.mu.Unlock()
		return
	}
	b.isRunning = false
	b.mu.Unlock()

	close(b.updates)
	b.wg.Wait()
	log.Println("[LogBatcher] Worker stopped")
}

// Queue adds an update to the buffer
func (b *LogBatcher) Queue(update LogUpdate) {
	select {
	case b.updates <- update:
	default:
		// Drop update if buffer is full to prevent blocking
		log.Printf("[LogBatcher] WARNING: Buffer full, dropping update for ID %d", update.ID)
	}
}

func (b *LogBatcher) worker() {
	defer b.wg.Done()

	buffer := make([]LogUpdate, 0, BatchSize)
	ticker := time.NewTicker(FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case update, ok := <-b.updates:
			if !ok {
				// Channel closed, flush remaining
				if len(buffer) > 0 {
					b.flush(buffer)
				}
				return
			}
			buffer = append(buffer, update)
			if len(buffer) >= BatchSize {
				b.flush(buffer)
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				b.flush(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

func (b *LogBatcher) flush(updates []LogUpdate) {
	if len(updates) == 0 {
		return
	}

	start := time.Now()
	
	// Create temporary table for bulk update
	// Note: MySQL doesn't have a direct "UPDATE FROM VALUES" syntax like PG.
	// We will use CASE WHEN syntax or INSERT ON DUPLICATE KEY UPDATE.
	// Since we are updating specific IDs, INSERT ON DUPLICATE is tricky unless we select all fields.
	// The most efficient standard SQL approach for bulk UPDATE by ID without selecting everything is:
	// INSERT INTO table (id, field) VALUES ... ON DUPLICATE KEY UPDATE field=VALUES(field)
    // But we need to make sure we don't overwrite fields with NULL if they weren't changed.
    // However, our struct has specific fields to update. 
    
    // Strategy: Construct a bulk UPDATE statement using CASE 
    // UPDATE apicall_call_log 
    // SET 
    //   status = CASE id 
    //     WHEN 1 THEN 'ANSWER'
    //     WHEN 2 THEN 'HANGUP'
    //   END,
    //   duracion = CASE id ... END
    // WHERE id IN (1, 2)

    ids := make([]string, len(updates))
    
    // Maps for constructing CASE statements
    statusCases := make([]string, 0, len(updates))
    duracionCases := make([]string, 0, len(updates))
    interaccionoCases := make([]string, 0, len(updates))
    
    // For nullable fields, we need to handle them carefully.
    // If pointer is nil, we iterate.
    dtmfCases := make([]string, 0, len(updates))
    dispositionCases := make([]string, 0, len(updates))
    uniqueidCases := make([]string, 0, len(updates))

    for i, u := range updates {
        ids[i] = fmt.Sprintf("%d", u.ID)
        
        statusCases = append(statusCases, fmt.Sprintf("WHEN %d THEN '%s'", u.ID, u.Status))
        duracionCases = append(duracionCases, fmt.Sprintf("WHEN %d THEN %d", u.ID, u.Duracion))
        
        interaccionoVal := "0"
        if u.Interacciono {
            interaccionoVal = "1"
        }
        interaccionoCases = append(interaccionoCases, fmt.Sprintf("WHEN %d THEN %s", u.ID, interaccionoVal))

        if u.DTMFMarcado != nil {
            dtmfCases = append(dtmfCases, fmt.Sprintf("WHEN %d THEN '%s'", u.ID, *u.DTMFMarcado))
        }

        if u.Disposition != nil {
             dispositionCases = append(dispositionCases, fmt.Sprintf("WHEN %d THEN '%s'", u.ID, *u.Disposition))
        }

        if u.Uniqueid != nil {
            uniqueidCases = append(uniqueidCases, fmt.Sprintf("WHEN %d THEN '%s'", u.ID, *u.Uniqueid))
        }
    }

    idList := strings.Join(ids, ",")
    
    var queryBuilder strings.Builder
    queryBuilder.WriteString("UPDATE apicall_call_log SET ")
    
    queryBuilder.WriteString(fmt.Sprintf("status = CASE id %s END, ", strings.Join(statusCases, " ")))
    queryBuilder.WriteString(fmt.Sprintf("duracion = CASE id %s END, ", strings.Join(duracionCases, " ")))
    queryBuilder.WriteString(fmt.Sprintf("interacciono = CASE id %s END", strings.Join(interaccionoCases, " ")))
    
    if len(dtmfCases) > 0 {
         queryBuilder.WriteString(fmt.Sprintf(", dtmf_marcado = CASE id %s ELSE dtmf_marcado END", strings.Join(dtmfCases, " ")))
    }
    
    if len(dispositionCases) > 0 {
         queryBuilder.WriteString(fmt.Sprintf(", disposition = CASE id %s ELSE disposition END", strings.Join(dispositionCases, " ")))
    }

    if len(uniqueidCases) > 0 {
         queryBuilder.WriteString(fmt.Sprintf(", uniqueid = CASE id %s ELSE uniqueid END", strings.Join(uniqueidCases, " ")))
    }

    queryBuilder.WriteString(fmt.Sprintf(" WHERE id IN (%s)", idList))

    query := queryBuilder.String()
    log.Printf("[LogBatcher] DEBUG Query: %s", query)
    
    _, err := b.db.Exec(query)
    if err != nil {
        log.Printf("[LogBatcher] ERROR flushing batch of %d items: %v", len(updates), err)
        // In a real system, we might want to retry or dump to a fallback file
    } else {
        log.Printf("[LogBatcher] Flushed %d updates in %v", len(updates), time.Since(start))
        // Sync campaign contacts based on updated call logs
        b.syncCampaignContacts(ids)
    }
}

// syncCampaignContacts updates campaign contacts based on finalized call logs
// It matches by telefono and proyecto_id to find the correct campaign contact
func (b *LogBatcher) syncCampaignContacts(logIDs []string) {
	if len(logIDs) == 0 {
		return
	}

	// This query updates campaign_contacts by joining with call_log
	// It maps call_log.status to campaign_contact.estado:
	// - ANSWERED/ANSWER -> completed
	// - NOANSWER/BUSY/FAILED/CONGESTION -> failed
	// - BLACKLISTED -> skipped
	// - Others stay as is (dialing contacts without matching final status)
	query := `
		UPDATE apicall_campaign_contacts cc
		INNER JOIN apicall_call_log cl ON cc.telefono = cl.telefono
		INNER JOIN apicall_campaigns c ON cc.campaign_id = c.id AND c.proyecto_id = cl.proyecto_id
		SET 
			cc.estado = CASE 
				WHEN cl.status IN ('ANSWERED', 'ANSWER', 'AMD_HUMAN', 'COMPLETED') THEN 'completed'
				WHEN cl.status IN ('NOANSWER', 'NO ANSWER', 'BUSY', 'FAILED', 'CONGESTION', 'CANCEL', 'TIMEOUT', 'AMD_MACHINE') THEN 'failed'
				WHEN cl.status = 'BLACKLISTED' THEN 'skipped'
				ELSE cc.estado
			END,
			cc.resultado = cl.status,
			cc.ultimo_intento = NOW()
		WHERE cl.id IN (` + strings.Join(logIDs, ",") + `)
		  AND cc.estado = 'dialing'
	`

	result, err := b.db.Exec(query)
	if err != nil {
		log.Printf("[LogBatcher] ERROR syncing campaign contacts: %v", err)
		return
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[LogBatcher] Synced %d campaign contacts", rows)
	}
}

