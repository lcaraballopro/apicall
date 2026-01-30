package asterisk

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"apicall/internal/database"
	"apicall/internal/dialer"
	"apicall/internal/smartcid"

	"github.com/google/uuid"
)

const (
	SpoolDir  = "/var/spool/asterisk/outgoing"
	TmpDir    = "/var/spool/asterisk/outgoing/.staging"
	QueueSize = 10000
)

// CallJob represents a call request
type CallJob struct {
	Proyecto   *database.Proyecto
	Telefono   string
	ContactID  int64  // ID del contacto de campaña (0 si no aplica)
	CampaignID int    // ID de la campaña (0 si no aplica)
}

var (
	jobQueue      chan CallJob
	workerRunning bool
	workerLimit   int
	workerRepo    *database.Repository
	scidGen       *smartcid.Generator
	channelPool   *dialer.ChannelPool       // Controls concurrent call limits
	callTracker   *dialer.ActiveCallTracker // Tracks active calls for correlation
	orphanCleaner *dialer.OrphanCallCleaner // Cleans up orphaned calls
)

// StartWorker initiates the spool worker
func StartWorker(maxCPS int, repo *database.Repository, pool *dialer.ChannelPool, tracker *dialer.ActiveCallTracker) {
	if workerRunning {
		return
	}

	// Ensure staging dir exists (essential for atomic moves on same filesystem)
	if err := os.MkdirAll(TmpDir, 0777); err != nil {
		log.Printf("[Spooler] ERROR CRITICO: No se pudo crear directorio de staging %s: %v", TmpDir, err)
	}


	// Try to load max_cps from DB
	cps := maxCPS
	if repo != nil {
		val, err := repo.GetConfig("max_cps")
		if err == nil && val != "" {
			if v, err := strconv.Atoi(val); err == nil && v > 0 {
				cps = v
				log.Printf("[Spooler] Loaded max_cps from DB: %d", cps)
			}
		}
	}

	if cps <= 0 {
		cps = 1
	}

	workerLimit = cps
	workerRepo = repo
	jobQueue = make(chan CallJob, QueueSize)

	// Use injected ChannelPool and Tracker
	channelPool = pool
	callTracker = tracker
	log.Printf("[Spooler] ChannelPool and CallTracker injected")

	// Start orphan cleaner
	orphanCleaner = dialer.NewOrphanCallCleaner(repo, channelPool, callTracker)
	orphanCleaner.Start()

	// Init SmartCID
	if repo.GetDB() != nil {
		scidGen = smartcid.NewGenerator(repo.GetDB())
		log.Printf("[Spooler] Smart CID Generator inicializado")
	} else {
		log.Printf("[Spooler] WARNING: No se pudo inicializar Smart CID Generator (DB es nil)")
	}

	workerRunning = true
	log.Printf("[Spooler] Worker iniciado (MaxCPS: %d)", cps)

	go processQueue()
}

// QueueCall queues a call (legacy, for non-campaign calls)
func QueueCall(proyecto *database.Proyecto, telefono string) {
	QueueCampaignCall(proyecto, telefono, 0, 0)
}

// QueueCampaignCall queues a call with campaign tracking
// Returns true if queued successfully, false if rejected (queue full or worker stopped)
func QueueCampaignCall(proyecto *database.Proyecto, telefono string, contactID int64, campaignID int) bool {
	if !workerRunning {
		log.Printf("[Spooler] Worker no iniciado, rechazando llamada a %s", telefono)
		return false
	}

	select {
	case jobQueue <- CallJob{Proyecto: proyecto, Telefono: telefono, ContactID: contactID, CampaignID: campaignID}:
		return true
	default:
		log.Printf("[Spooler] Cola llena, rechazando llamada a %s", telefono)
		return false
	}
}

func processQueue() {
	var currentTPS int = workerLimit
	if currentTPS <= 0 {
		currentTPS = 1
	}

	interval := time.Second / time.Duration(currentTPS)
	ticker := time.NewTicker(interval)
	// No defer ticker.Stop() here because we reassign it

	// Config watcher ticker (check every 5 seconds)
	configTicker := time.NewTicker(5 * time.Second)
	defer configTicker.Stop()

	log.Printf("[Spooler] Processing loop started at %d CPS", currentTPS)

	for {
		select {
		case job, ok := <-jobQueue:
			if !ok {
				ticker.Stop()
				return
			}
			<-ticker.C
			generateCallFile(job)
		case <-configTicker.C:
			if workerRepo != nil {
				val, err := workerRepo.GetConfig("max_cps")
				if err == nil && val != "" {
					newCPS, err := strconv.Atoi(val)
					if err == nil && newCPS > 0 && newCPS != currentTPS {
						log.Printf("[Spooler] Updating CPS from %d to %d", currentTPS, newCPS)
						currentTPS = newCPS
						ticker.Stop()
						interval = time.Second / time.Duration(currentTPS)
						ticker = time.NewTicker(interval)
					}
				}
			}
		}
	}
}

func generateCallFile(job CallJob) {
	uniqueID := uuid.New().String()
	fileName := fmt.Sprintf("apicall_%d_%s_%s.call", job.Proyecto.ID, job.Telefono, uniqueID)
	tmpPath := filepath.Join(TmpDir, fileName)
	destPath := filepath.Join(SpoolDir, fileName)

	// Smart Caller ID Determination
	cid := job.Proyecto.CallerID
	if scidGen != nil && job.Proyecto.SmartCIDActive {
		generatedCID := scidGen.GetCallerID(job.Telefono, cid, job.Proyecto.SmartCIDActive)
		log.Printf("[Spooler] Smart CID: Proyecto=%d, Destino=%s, Original=%s, Generado=%s",
			job.Proyecto.ID, job.Telefono, cid, generatedCID)
		cid = generatedCID
	} else {
		log.Printf("[Spooler] Usando CID estático: Proyecto=%d, CID=%s (SmartGen=%v, SmartActive=%v)",
			job.Proyecto.ID, cid, scidGen != nil, job.Proyecto.SmartCIDActive)
	}

	// Create DB Log
	var campaignID *int
	if job.CampaignID > 0 {
		cid := job.CampaignID
		campaignID = &cid
	}

	callLog := &database.CallLog{
		ProyectoID:   job.Proyecto.ID,
		Telefono:     job.Telefono,
		Status:       "DIALING",
		Interacciono: false,
		CallerIDUsed: cid,
		CampaignID:   campaignID,
	}

	logID, err := workerRepo.CreateCallLog(callLog)
	if err != nil {
		log.Printf("[Spooler] Error creando log DB: %v", err)
		return
	}

	// Create .call content
	// Use SIP/<trunk>/<number> format instead of Local
	// Add prefix if configured
	dialNumber := job.Telefono
	if job.Proyecto.PrefijoSalida != "" {
		dialNumber = job.Proyecto.PrefijoSalida + job.Telefono
		log.Printf("[Spooler] Agregando prefijo: %s + %s = %s",
			job.Proyecto.PrefijoSalida, job.Telefono, dialNumber)
	}

	// LOAD BALANCING LOGIC
	var selectedTrunk string

	// 1. Try relational table
	if workerRepo != nil {
		names, err := workerRepo.GetTroncalesNamesByProyecto(job.Proyecto.ID)
		if err == nil && len(names) > 0 {
			selectedTrunk = names[rand.Intn(len(names))]
			if len(names) > 1 {
				log.Printf("[Spooler] Load Balancing (Table): Selected trunk '%s' from list %v", selectedTrunk, names)
			}
		}
	}

	// 2. Fallback to comma-separated string
	if selectedTrunk == "" {
		trunks := strings.Split(job.Proyecto.TroncalSalida, ",")
		selectedTrunk = strings.TrimSpace(trunks[0])
		if len(trunks) > 1 {
			selectedTrunk = strings.TrimSpace(trunks[rand.Intn(len(trunks))])
			log.Printf("[Spooler] Load Balancing (Legacy): Selected trunk '%s' from '%s'", selectedTrunk, job.Proyecto.TroncalSalida)
		}
	}

	// CHECK CHANNEL LIMITS before proceeding
	if channelPool != nil && !channelPool.Acquire(selectedTrunk) {
		log.Printf("[Spooler] Channel limit reached, rejecting call to %s (trunk: %s)", job.Telefono, selectedTrunk)
		workerRepo.UpdateCallLog(logID, nil, nil, nil, false, "CHANNEL_LIMIT", 0)
		// Update contact status if applicable
		if job.ContactID > 0 {
			pending := "pending" // Return to pending so it can be retried
			workerRepo.UpdateContactStatus(job.ContactID, pending, nil)
		}
		return
	}

	content := fmt.Sprintf(`Channel: SIP/%s/%s
CallerID: "%s" <%s>
MaxRetries: %d
RetryTime: %d
WaitTime: 45
Context: apicall_context
Extension: s
Priority: 1
Set: APICALL_LOG_ID=%d
Set: APICALL_PROYECTO_ID=%d
Set: APICALL_TELEFONO=%s
Set: APICALL_UNIQUEID=%s
Set: APICALL_CONTACT_ID=%d
Set: APICALL_CAMPAIGN_ID=%d
Archive: yes
`, selectedTrunk, dialNumber,
		job.Proyecto.Nombre, cid,
		job.Proyecto.MaxRetries,
		job.Proyecto.RetryTime,
		logID,
		job.Proyecto.ID,
		job.Telefono,
		uniqueID,
		job.ContactID,
		job.CampaignID,
	)

	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		log.Printf("[Spooler] Error escribiendo archivo tmp: %v", err)
		workerRepo.UpdateCallLog(logID, nil, nil, nil, false, "SPOOL_ERROR", 0)
		return
	}

	// Register active call for tracking BEFORE moving file
	// This prevents race condition where Asterisk executes and sends event
	// before we track it
	if callTracker != nil {
		callTracker.Add(&dialer.ActiveCall{
			UniqueID:   uniqueID,
			LogID:      logID,
			ContactID:  job.ContactID,
			CampaignID: job.CampaignID,
			ProyectoID: job.Proyecto.ID,
			Trunk:      selectedTrunk,
			Telefono:   job.Telefono,
			StartTime:  time.Now(),
		})
	}

	// Atomic Move
	if err := os.Rename(tmpPath, destPath); err != nil {
		log.Printf("[Spooler] Error moviendo archivo a spool: %v", err)
		os.Remove(tmpPath)
		workerRepo.UpdateCallLog(logID, nil, nil, nil, false, "SPOOL_ERROR", 0)
		
		// Rollback tracking and limits
		if callTracker != nil {
			callTracker.Remove(uniqueID)
		}
		if channelPool != nil {
			channelPool.Release(selectedTrunk)
		}
		return
	}
}

// ReleaseChannel releases a channel slot when a call ends
// Called by AMI event handler when a call completes
func ReleaseChannel(uniqueID string) {
	if callTracker == nil {
		return
	}
	
	call := callTracker.Remove(uniqueID)
	if call != nil && channelPool != nil {
		channelPool.Release(call.Trunk)
	}
}

// GetActiveCall retrieves an active call by uniqueID
func GetActiveCall(uniqueID string) *dialer.ActiveCall {
	if callTracker == nil {
		return nil
	}
	return callTracker.Get(uniqueID)
}

// GetChannelStats returns current channel pool statistics
func GetChannelStats() *dialer.PoolStats {
	if channelPool == nil {
		return nil
	}
	stats := channelPool.Stats()
	return &stats
}

// GetActiveCallCount returns the number of active calls
func GetActiveCallCount() int {
	if callTracker == nil {
		return 0
	}
	return callTracker.Count()
}
