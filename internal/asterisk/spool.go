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
	Proyecto *database.Proyecto
	Telefono string
}

var (
	jobQueue      chan CallJob
	workerRunning bool
	workerLimit   int
	workerRepo    *database.Repository
	scidGen       *smartcid.Generator
)

// StartWorker initiates the spool worker
func StartWorker(maxCPS int, repo *database.Repository) {
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

// QueueCall queues a call
func QueueCall(proyecto *database.Proyecto, telefono string) {
	if !workerRunning {
		log.Printf("[Spooler] Worker no iniciado, rechazando llamada a %s", telefono)
		return
	}

	select {
	case jobQueue <- CallJob{Proyecto: proyecto, Telefono: telefono}:
	default:
		log.Printf("[Spooler] Cola llena, rechazando llamada a %s", telefono)
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
	callLog := &database.CallLog{
		ProyectoID:   job.Proyecto.ID,
		Telefono:     job.Telefono,
		Status:       "DIALING",
		Interacciono: false,
		CallerIDUsed: cid,
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
Archive: yes
`, selectedTrunk, dialNumber,
		job.Proyecto.Nombre, cid,
		job.Proyecto.MaxRetries,
		job.Proyecto.RetryTime,
		logID,
		job.Proyecto.ID,
		job.Telefono,
		uniqueID,
	)

	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		log.Printf("[Spooler] Error escribiendo archivo tmp: %v", err)
		workerRepo.UpdateCallLog(logID, nil, nil, false, "SPOOL_ERROR", 0)
		return
	}

	// Atomic Move
	if err := os.Rename(tmpPath, destPath); err != nil {
		log.Printf("[Spooler] Error moviendo archivo a spool: %v", err)
		os.Remove(tmpPath)
		workerRepo.UpdateCallLog(logID, nil, nil, false, "SPOOL_ERROR", 0)
		return
	}
}
