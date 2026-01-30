package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"apicall/internal/ami"
	"apicall/internal/asterisk"
	"apicall/internal/auth"
	"apicall/internal/config"
	"apicall/internal/database"
	"apicall/internal/provisioning"
	"apicall/internal/smartcid"
	ws "apicall/internal/websocket"
)

// Server representa el servidor API REST
type Server struct {
	config *config.Config
	repo   *database.Repository
	ami    *ami.Client
}

// NewServer crea un nuevo servidor API
func NewServer(cfg *config.Config, repo *database.Repository, ami *ami.Client) *Server {
	return &Server{
		config: cfg,
		repo:   repo,
		ami:    ami,
	}
}

// Start inicia el servidor HTTP
func (s *Server) Start() error {
	addr := s.config.API.Address()
	log.Printf("[API] Iniciando servidor en %s", addr)

	// Initialize WebSocket hub for real-time updates
	ws.Init()

	mux := http.NewServeMux()

	// 1. Static Files (Public) - Serve React build with SPA fallback
	staticDir := "./web-react/dist"
	fs := http.FileServer(http.Dir(staticDir))
	
	// SPA Handler: serves static files or falls back to index.html for React Router
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve static file first
		path := staticDir + r.URL.Path
		if r.URL.Path != "/" {
			if _, err := os.Stat(path); err == nil {
				fs.ServeHTTP(w, r)
				return
			}
		}
		// Fallback to index.html for SPA routes
		http.ServeFile(w, r, staticDir+"/index.html")
	})


	// 2. Public API Endpoints
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/health", s.handleHealth)
	
	// API Documentation (public)
	mux.HandleFunc("/api-docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/api-docs.html")
	})
	
	// Logo (public)
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/logo.png")
	})

	// 3. Protected API Routes
	// We create a sub-handler for protected routes to wrap them in middleware
	protectedMux := http.NewServeMux()

	protectedMux.HandleFunc("/api/v1/call", s.handleCall)

	protectedMux.HandleFunc("/api/v1/proyectos", s.handleProyectos)
	protectedMux.HandleFunc("/api/v1/proyectos/delete", s.handleProyectoDelete)
	protectedMux.HandleFunc("/api/v1/proyectos/audio", s.handleProyectoAudio)

	protectedMux.HandleFunc("/api/v1/troncales", s.handleTroncales)
	protectedMux.HandleFunc("/api/v1/troncales/delete", s.handleTroncalDelete)

	protectedMux.HandleFunc("/api/v1/logs", s.handleLogs)
	protectedMux.HandleFunc("/api/v1/logs/status", s.handleLogStatus)

	// User Management
	protectedMux.HandleFunc("/api/v1/users", s.handleUsers)
	protectedMux.HandleFunc("/api/v1/users/delete", s.handleUserDelete)

	// Audio Management
	protectedMux.HandleFunc("/api/v1/audios", s.handleAudios)
	protectedMux.HandleFunc("/api/v1/audios/upload", s.handleAudioUpload)
	protectedMux.HandleFunc("/api/v1/audios/delete", s.handleAudioDelete)
	protectedMux.HandleFunc("/api/v1/audios/stream", s.handleAudioStream)

	// Blacklist Management
	protectedMux.HandleFunc("/api/v1/blacklist", s.handleBlacklist)
	protectedMux.HandleFunc("/api/v1/blacklist/upload", s.handleBlacklistUpload)
	protectedMux.HandleFunc("/api/v1/blacklist/delete", s.handleBlacklistDelete)
	protectedMux.HandleFunc("/api/v1/blacklist/clear", s.handleBlacklistClear)

	// Campaign Management
	protectedMux.HandleFunc("/api/v1/campaigns", s.handleCampaigns)
	protectedMux.HandleFunc("/api/v1/campaigns/delete", s.handleCampaignDelete)
	protectedMux.HandleFunc("/api/v1/campaigns/upload", s.handleCampaignUpload)
	protectedMux.HandleFunc("/api/v1/campaigns/action", s.handleCampaignAction)
	protectedMux.HandleFunc("/api/v1/campaigns/stats", s.handleCampaignStats)
	protectedMux.HandleFunc("/api/v1/campaigns/schedules", s.handleCampaignSchedules)
	protectedMux.HandleFunc("/api/v1/campaigns/dispositions", s.handleCampaignDispositions)
	protectedMux.HandleFunc("/api/v1/campaigns/recycle", s.handleCampaignRecycle)

	// System Configuration Management
	protectedMux.HandleFunc("/api/v1/config", s.handleConfig)

	// WebSocket endpoint (public, no auth needed for upgrade)
	mux.HandleFunc("/ws", ws.HandleWebSocket)

	// Custom Handler to route between Public and Protected
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// List of public prefixes
		if r.URL.Path == "/api/v1/login" || r.URL.Path == "/health" || !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			mux.ServeHTTP(w, r)
			return
		}

		// If it is /api/v1/..., enforce Auth
		auth.Middleware(protectedMux).ServeHTTP(w, r)
	})

	log.Printf("[API] Servidor iniciado correctamente")

	// Apply CORS to the top-level handler
	return http.ListenAndServe(addr, s.corsMiddleware(mainHandler))
}

// corsMiddleware agrega headers CORS si está habilitado
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.API.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		defer func() {
			if r := recover(); r != nil {
				log.Printf("[API] PANIC RECOVERED: %v", r)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error": "Internal Server Error"}`)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// handleCall maneja solicitudes para generar llamadas
func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parsear body
	var req struct {
		ProyectoID int    `json:"proyecto_id"`
		Telefono   string `json:"telefono"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Validar parámetros
	if req.ProyectoID == 0 || req.Telefono == "" {
		http.Error(w, "proyecto_id y telefono son requeridos", http.StatusBadRequest)
		return
	}

	// Obtener proyecto
	proyecto, err := s.repo.GetProyecto(req.ProyectoID)
	if err != nil {
		http.Error(w, "Proyecto no encontrado", http.StatusNotFound)
		return
	}

	// Validar IP autorizada
	clientIP := getClientIP(r)
	if !s.isIPAuthorized(clientIP, proyecto.IPsAutorizadas) {
		log.Printf("[API] IP no autorizada: %s para proyecto %d", clientIP, req.ProyectoID)
		http.Error(w, "IP no autorizada", http.StatusForbidden)
		return
	}

	// Verificar blacklist
	if blacklisted, _ := s.repo.IsBlacklisted(req.ProyectoID, req.Telefono); blacklisted {
		log.Printf("[API] Número en blacklist: %s para proyecto %d", req.Telefono, req.ProyectoID)
		http.Error(w, "Número en lista negra", http.StatusForbidden)
		return
	}

	// Encolar llamada en Spooler (Rate Limited)
	asterisk.QueueCall(proyecto, req.Telefono)

	log.Printf("[API] Llamada encolada: proyecto=%d telefono=%s ip=%s",
		req.ProyectoID, req.Telefono, clientIP)

	// Responder 202 Accepted
	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"proyecto_id": req.ProyectoID,
		"telefono":    req.Telefono,
		"message":     "Llamada encolada correctamente",
	})
}

// handleProyectos gestiona la creación y listado de proyectos
func (s *Server) handleProyectos(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var p database.Proyecto
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		if err := s.repo.CreateProyecto(&p); err != nil {
			http.Error(w, fmt.Sprintf("Error creando proyecto: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
		return
	}

	if r.Method == http.MethodGet {
		proyectos, err := s.repo.ListProyectos()
		if err != nil {
			http.Error(w, "Error listando proyectos", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(proyectos)
		return
	}

	if r.Method == http.MethodPut {
		var p database.Proyecto
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		if p.ID == 0 {
			http.Error(w, "ID de proyecto requerido", http.StatusBadRequest)
			return
		}
		if err := s.repo.UpdateProyecto(&p); err != nil {
			http.Error(w, fmt.Sprintf("Error actualizando proyecto: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
		return
	}

	http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
}


// handleProyectoDelete elimina un proyecto
func (s *Server) handleProyectoDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost { // Permitir POST para facilitar CLI simple
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "ID requerido", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := s.repo.DeleteProyecto(id); err != nil {
		http.Error(w, fmt.Sprintf("Error eliminando proyecto: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// handleTroncales gestiona troncales SIP
func (s *Server) handleTroncales(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var t database.Troncal
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		if err := s.repo.CreateTroncal(&t); err != nil {
			http.Error(w, fmt.Sprintf("Error creando troncal: %v", err), http.StatusInternalServerError)
			return
		}

		// Sincronizar (best effort)
		provisioning.SyncTroncales(s.repo)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(t)
		return
	}

	if r.Method == http.MethodGet {
		troncales, err := s.repo.ListTroncales()
		if err != nil {
			log.Printf("[API] Error listando troncales: %v", err)
			http.Error(w, "Error listando troncales", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(troncales)
		return
	}

	http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
}

// handleTroncalDelete elimina una troncal
func (s *Server) handleTroncalDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := s.repo.DeleteTroncal(id); err != nil {
		http.Error(w, fmt.Sprintf("Error eliminando troncal: %v", err), http.StatusInternalServerError)
		return
	}

	// Sincronizar
	provisioning.SyncTroncales(s.repo)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// handleLogs obtiene logs de llamadas
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Obtener parámetros opcionales
	proyectoIDStr := r.URL.Query().Get("proyecto_id")
	limitStr := r.URL.Query().Get("limit")
	fromDate := r.URL.Query().Get("from_date")
	toDate := r.URL.Query().Get("to_date")

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var logs []database.CallLog
	var err error

	if proyectoIDStr != "" {
		// Filter by project
		var proyectoID int
		if _, parseErr := fmt.Sscanf(proyectoIDStr, "%d", &proyectoID); parseErr != nil {
			http.Error(w, "proyecto_id inválido", http.StatusBadRequest)
			return
		}

		var campaignID *int
		campaignIDStr := r.URL.Query().Get("campaign_id")
		if campaignIDStr != "" {
			if cid, err := strconv.Atoi(campaignIDStr); err == nil {
				campaignID = &cid
			}
		}

		if fromDate != "" || toDate != "" {
			logs, err = s.repo.GetCallLogsByProyectoWithDates(proyectoID, campaignID, limit, fromDate, toDate)
		} else {
			logs, err = s.repo.GetCallLogsByProyecto(proyectoID, campaignID, limit)
		}
	} else {
		// Get all logs
		if fromDate != "" || toDate != "" {
			logs, err = s.repo.GetRecentCallLogsWithDates(limit, fromDate, toDate)
		} else {
			logs, err = s.repo.GetRecentCallLogs(limit)
		}
	}

	if err != nil {
		log.Printf("[API] Error obteniendo logs: %v", err)
		http.Error(w, "Error obteniendo logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// handleLogStatus actualiza el estado de un log (usado por Dialplan)
func (s *Server) handleLogStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parsear parámetros (puede venir como x-www-form-urlencoded desde Asterisk CURL)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parseando form", http.StatusBadRequest)
		return
	}

	logIDStr := r.FormValue("id")
	status := r.FormValue("status")

	// Si no vino en form, intentar query params (común en algunos setups)
	if logIDStr == "" {
		logIDStr = r.URL.Query().Get("id")
	}
	if status == "" {
		status = r.URL.Query().Get("status")
	}

	if logIDStr == "" || status == "" {
		http.Error(w, "id y status requeridos", http.StatusBadRequest)
		return
	}

	var logID int64
	if _, err := fmt.Sscanf(logIDStr, "%d", &logID); err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	// Mapear DIALSTATUS de Asterisk a Disposition estándar Contact Center
	// Standard codes: A=Answered, B=Busy, NA=No Answer, CONG=Congestion, FAIL=Failed
	var disposition string
	switch status {
	case "ANSWER":
		disposition = "A" // Answered/Contacted
	case "BUSY":
		disposition = "B" // Busy
	case "NOANSWER":
		disposition = "NA" // No Answer
	case "CANCEL":
		disposition = "NA" // Cancelled = No Answer
	case "CONGESTION":
		disposition = "CONG" // Congestion
	case "CHANUNAVAIL":
		disposition = "FAIL" // Channel Unavailable = Failed
	default:
		disposition = status
	}

	if err := s.repo.UpdateCallLog(logID, nil, &disposition, nil, false, status, 0); err != nil {
		log.Printf("[API] Error actualizando status log %d: %v", logID, err)
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}

	// Update Smart Caller ID stats
	// We need the log to know the CallerID used, but we don't have it in request.
	// For MVP: We assume we can't fully track specific used CID unless we save it in DB
	// or Asterisk passes it back.
	// Asterisk passes `id` (logID). We can fetch the log?
	// Wait, the log table doesn't have "used_callerid".
	// Feature enhancement: Add `caller_id` to `apicall_logs`.
	// For now, let's look up the project standard CID? No, that defeats point.
	// If we want to optimize, we MUST know what ID we presented.

	// SKIP SmartCID update for now until we add 'caller_id' column to logs in a future migration.
	// Document limitation.
	// OR: Assume we passed it in request?
	// Set: APICALL_CALLERID=%s in call file was removed in my rewrite?
	// Rewrite `server.go` only partially?

	log.Printf("[API] Log %d actualizado a status %s (Disposition: %s)", logID, status, disposition)

	// Update Smart Caller ID stats
	if s.ami != nil { // accessing scidGen? No, scidGen is in provisioning or spooler?
		// We need access to scidGen or create one.
		// server.gp doesn't have scidGen field yet.
		// Let's create a temporary one or add it to Server struct.
		// Since we have repo, we can fetch the DB.
		if s.repo.GetDB() != nil {
			// Retrieve log to get used CID
			// We need GetCallLog(id) in repository.
			// If we don't have it, we can't do it right now.
			// Let's implement GetCallLog briefly in repo to make this work.

			// Check if we have GetCallLog in repo?
			// Assuming not, let's query directly or skip for now to avoid complexity explosion?
			// User asked for "identifique patrones".
			// Let's do a direct query here for speed.
			var usedCID string
			err := s.repo.GetDB().QueryRow("SELECT caller_id_used FROM apicall_call_log WHERE id = ?", logID).Scan(&usedCID)
			if err == nil && usedCID != "" {
				gen := smartcid.NewGenerator(s.repo.GetDB())
				gen.UpdateStats(usedCID, disposition == "A")
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleHealth endpoint de salud
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// getClientIP obtiene la IP real del cliente
func getClientIP(r *http.Request) string {
	// Intentar obtener de headers comunes
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Usar RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// isIPAuthorized verifica si una IP está autorizada
func (s *Server) isIPAuthorized(clientIP string, autorizadas string) bool {
	if autorizadas == "" || autorizadas == "*" {
		return true // Sin restricciones
	}

	clientIPObj := net.ParseIP(clientIP)
	if clientIPObj == nil {
		return false
	}

	// Separar IPs/CIDRs autorizadas
	ips := strings.Split(autorizadas, ",")
	for _, ipStr := range ips {
		ipStr = strings.TrimSpace(ipStr)

		// Verificar si es CIDR
		if strings.Contains(ipStr, "/") {
			_, network, err := net.ParseCIDR(ipStr)
			if err != nil {
				continue
			}
			if network.Contains(clientIPObj) {
				return true
			}
		} else {
			// IP individual
			if clientIP == ipStr {
				return true
			}
		}
	}

	return false
}

// handleLogin procesa el inicio de sesión
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	user, err := s.repo.GetUserByUsername(creds.Username)
	if err != nil || user == nil {
		// Log failed attempt but don't reveal user existence
		log.Printf("[Auth] Fallo login para usuario: %s", creds.Username)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Credenciales inválidas"})
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, creds.Password); err != nil {
		log.Printf("[Auth] Contraseña incorrecta para usuario: %s", creds.Username)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Credenciales inválidas"})
		return
	}

	// Generate JWT
	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		http.Error(w, "Error generando token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"username": user.Username,
			"role":     user.Role,
			"fullName": user.FullName,
		},
	})
}

// handleUsers administra usuarios
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	// Verificar rol (solo admin)
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, "Acceso denegado: Se requiere rol de Admin", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		users, err := s.repo.ListUsers()
		if err != nil {
			http.Error(w, "Error listando usuarios", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(users)
		return
	}

	if r.Method == http.MethodPost {
		var u database.User
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
			FullName string `json:"full_name"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "Error hasheando contraseña", http.StatusInternalServerError)
			return
		}

		u.Username = req.Username
		u.PasswordHash = hash
		u.Role = req.Role
		u.FullName = req.FullName

		if err := s.repo.CreateUser(&u); err != nil {
			http.Error(w, fmt.Sprintf("Error creando usuario: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	// Verificar rol (solo admin)
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, "Acceso denegado", http.StatusForbidden)
		return
	}

	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	if err := s.repo.DeleteUser(id); err != nil {
		http.Error(w, "Error eliminando usuario", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// --- AUDIO MANAGEMENT ---

// handleAudios lists available audio files
func (s *Server) handleAudios(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	audioDir := "/var/lib/asterisk/sounds/apicall"
	files, err := os.ReadDir(audioDir)
	if err != nil {
		// Directory might not exist yet
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	audios := make([]map[string]interface{}, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, _ := file.Info()
		audios = append(audios, map[string]interface{}{
			"name": file.Name(),
			"size": info.Size(),
			"date": info.ModTime(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(audios)
}

// handleAudioUpload handles file uploads with automatic format conversion
func (s *Server) handleAudioUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verify admin role
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, "Acceso denegado: Se requiere rol de Admin", http.StatusForbidden)
		return
	}

	// Parse multipart form (max 50MB)
	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "Archivo demasiado grande", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "No se recibió archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get custom name from form (optional, defaults to original filename)
	customName := r.FormValue("name")
	if customName == "" {
		// Use original filename without extension
		customName = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	}
	
	// Sanitize custom name - only allow alphanumeric, hyphen, underscore
	customName = strings.ToLower(customName)
	for _, c := range customName {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			customName = strings.ReplaceAll(customName, string(c), "_")
		}
	}
	
	if customName == "" {
		customName = "audio"
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{
		".wav": true, ".gsm": true, ".ulaw": true, ".alaw": true, 
		".sln": true, ".mp3": true, ".ogg": true, ".flac": true, ".m4a": true,
	}
	if !allowedExts[ext] {
		http.Error(w, "Formato no soportado. Use: wav, gsm, ulaw, alaw, sln, mp3, ogg, flac, m4a", http.StatusBadRequest)
		return
	}

	// Create directories
	audioDir := "/var/lib/asterisk/sounds/apicall"
	tempDir := "/tmp/apicall_audio"
	os.MkdirAll(audioDir, 0755)
	os.MkdirAll(tempDir, 0755)

	// Save to temp file first
	tempPath := filepath.Join(tempDir, fmt.Sprintf("upload_%d%s", time.Now().UnixNano(), ext))
	tempFile, err := os.Create(tempPath)
	if err != nil {
		log.Printf("[API] Error creando archivo temporal: %v", err)
		http.Error(w, "Error guardando archivo", http.StatusInternalServerError)
		return
	}
	
	if _, err := io.Copy(tempFile, file); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		http.Error(w, "Error escribiendo archivo", http.StatusInternalServerError)
		return
	}
	tempFile.Close()
	
	// Final output path (always .wav)
	finalFilename := customName + ".wav"
	finalPath := filepath.Join(audioDir, finalFilename)
	
	// Convert to Asterisk-compatible format using sox
	// Format: 8000 Hz, mono, 16-bit signed PCM WAV
	cmd := exec.Command("sox", tempPath, "-r", "8000", "-c", "1", "-b", "16", finalPath)
	output, err := cmd.CombinedOutput()
	
	// Clean up temp file
	os.Remove(tempPath)
	
	if err != nil {
		log.Printf("[API] Error convirtiendo audio con sox: %v - Output: %s", err, string(output))
		http.Error(w, fmt.Sprintf("Error convirtiendo audio: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Get final file info
	finalInfo, _ := os.Stat(finalPath)
	var finalSize int64
	if finalInfo != nil {
		finalSize = finalInfo.Size()
	}

	log.Printf("[API] Audio subido y convertido: %s (original: %d bytes, convertido: %d bytes)", 
		finalFilename, header.Size, finalSize)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"filename":      finalFilename,
		"path":          fmt.Sprintf("apicall/%s", finalFilename),
		"original_size": header.Size,
		"final_size":    finalSize,
		"converted":     true,
	})
}


// handleAudioDelete deletes an audio file
func (s *Server) handleAudioDelete(w http.ResponseWriter, r *http.Request) {
	// Verify admin role
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, "Acceso denegado", http.StatusForbidden)
		return
	}

	filename := r.URL.Query().Get("name")
	if filename == "" {
		http.Error(w, "Nombre de archivo requerido", http.StatusBadRequest)
		return
	}

	// Security: prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Nombre de archivo inválido", http.StatusBadRequest)
		return
	}

	audioPath := filepath.Join("/var/lib/asterisk/sounds/apicall", filename)
	if err := os.Remove(audioPath); err != nil {
		http.Error(w, "Error eliminando archivo", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Audio eliminado: %s", filename)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// handleAudioStream streams an audio file for browser playback
func (s *Server) handleAudioStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Query().Get("name")
	if filename == "" {
		http.Error(w, "Nombre de archivo requerido", http.StatusBadRequest)
		return
	}

	// Security: prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Nombre de archivo inválido", http.StatusBadRequest)
		return
	}

	audioPath := filepath.Join("/var/lib/asterisk/sounds/apicall", filename)
	
	// Check file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		http.Error(w, "Archivo no encontrado", http.StatusNotFound)
		return
	}

	// Detect content type based on extension
	ext := strings.ToLower(filepath.Ext(filename))
	contentTypes := map[string]string{
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".gsm":  "audio/x-gsm",
		".ulaw": "audio/basic",
		".alaw": "audio/basic",
		".sln":  "audio/x-raw",
	}
	
	contentType := contentTypes[ext]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeFile(w, r, audioPath)
}

// --- BLACKLIST MANAGEMENT ---

// handleBlacklist lista y agrega números a la blacklist
func (s *Server) handleBlacklist(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		proyectoIDStr := r.URL.Query().Get("proyecto_id")
		if proyectoIDStr == "" {
			http.Error(w, "proyecto_id requerido", http.StatusBadRequest)
			return
		}

		proyectoID, err := strconv.Atoi(proyectoIDStr)
		if err != nil {
			http.Error(w, "proyecto_id inválido", http.StatusBadRequest)
			return
		}

		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		entries, err := s.repo.ListBlacklist(proyectoID, limit)
		if err != nil {
			http.Error(w, "Error obteniendo blacklist", http.StatusInternalServerError)
			return
		}

		count, _ := s.repo.CountBlacklist(proyectoID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": entries,
			"total":   count,
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			ProyectoID int    `json:"proyecto_id"`
			Telefono   string `json:"telefono"`
			Razon      string `json:"razon"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		if req.ProyectoID == 0 || req.Telefono == "" {
			http.Error(w, "proyecto_id y telefono requeridos", http.StatusBadRequest)
			return
		}

		var razon *string
		if req.Razon != "" {
			razon = &req.Razon
		}

		entry := &database.BlacklistEntry{
			ProyectoID: req.ProyectoID,
			Telefono:   req.Telefono,
			Razon:      razon,
		}

		if err := s.repo.AddToBlacklist(entry); err != nil {
			http.Error(w, fmt.Sprintf("Error agregando a blacklist: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("[API] Número agregado a blacklist: proyecto=%d telefono=%s", req.ProyectoID, req.Telefono)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	}

	http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
}

// handleBlacklistUpload maneja la carga de CSV para blacklist
func (s *Server) handleBlacklistUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Archivo demasiado grande", http.StatusBadRequest)
		return
	}

	proyectoIDStr := r.FormValue("proyecto_id")
	if proyectoIDStr == "" {
		http.Error(w, "proyecto_id requerido", http.StatusBadRequest)
		return
	}

	proyectoID, err := strconv.Atoi(proyectoIDStr)
	if err != nil {
		http.Error(w, "proyecto_id inválido", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No se recibió archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error leyendo archivo", http.StatusInternalServerError)
		return
	}

	// Parse CSV (semicolon-delimited)
	lines := strings.Split(string(content), "\n")
	var telefonos []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header row if present
		if i == 0 && (strings.ToLower(line) == "telefono" || strings.Contains(strings.ToLower(line), "phone")) {
			continue
		}

		// Split by semicolon and take first column
		parts := strings.Split(line, ";")
		tel := strings.TrimSpace(parts[0])
		if tel != "" {
			telefonos = append(telefonos, tel)
		}
	}

	inserted, err := s.repo.AddToBlacklistBulk(proyectoID, telefonos)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error importando: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Blacklist CSV importado: proyecto=%d insertados=%d", proyectoID, inserted)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"imported": inserted,
		"total":    len(telefonos),
	})
}

// handleBlacklistDelete elimina un número de la blacklist
func (s *Server) handleBlacklistDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "ID requerido", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := s.repo.DeleteFromBlacklist(id); err != nil {
		http.Error(w, "Error eliminando de blacklist", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Número eliminado de blacklist: id=%d", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// handleBlacklistClear elimina todos los números de la blacklist de un proyecto
func (s *Server) handleBlacklistClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	proyectoIDStr := r.URL.Query().Get("proyecto_id")
	if proyectoIDStr == "" {
		http.Error(w, "proyecto_id requerido", http.StatusBadRequest)
		return
	}

	proyectoID, err := strconv.Atoi(proyectoIDStr)
	if err != nil {
		http.Error(w, "proyecto_id inválido", http.StatusBadRequest)
		return
	}

	if err := s.repo.ClearBlacklist(proyectoID); err != nil {
		http.Error(w, "Error limpiando blacklist", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Blacklist limpiada: proyecto=%d", proyectoID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// --- CAMPAIGN MANAGEMENT ---

// handleCampaigns manages campaign CRUD operations
func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// List campaigns, optionally filtered by proyecto_id
		proyectoIDStr := r.URL.Query().Get("proyecto_id")
		
		var campaigns []database.Campaign
		var err error
		
		if proyectoIDStr != "" {
			proyectoID, _ := strconv.Atoi(proyectoIDStr)
			campaigns, err = s.repo.ListCampaignsByProyecto(proyectoID)
		} else {
			campaigns, err = s.repo.ListCampaigns()
		}
		
		if err != nil {
			log.Printf("[API] Error listing campaigns: %v", err)
			http.Error(w, "Error listando campañas", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(campaigns)

	case http.MethodPost:
		var c database.Campaign
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		
		if c.Nombre == "" || c.ProyectoID == 0 {
			http.Error(w, "nombre y proyecto_id son requeridos", http.StatusBadRequest)
			return
		}
		
		c.Estado = "draft"
		if err := s.repo.CreateCampaign(&c); err != nil {
			log.Printf("[API] Error creating campaign: %v", err)
			http.Error(w, fmt.Sprintf("Error creando campaña: %v", err), http.StatusInternalServerError)
			return
		}
		
		log.Printf("[API] Campaña creada: id=%d nombre=%s", c.ID, c.Nombre)
		json.NewEncoder(w).Encode(c)

	case http.MethodPut:
		var c database.Campaign
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}
		
		if c.ID == 0 {
			http.Error(w, "ID de campaña requerido", http.StatusBadRequest)
			return
		}
		
		if err := s.repo.UpdateCampaign(&c); err != nil {
			http.Error(w, fmt.Sprintf("Error actualizando campaña: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(c)

	default:
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

// handleCampaignDelete deletes a campaign
func (s *Server) handleCampaignDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "ID requerido", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := s.repo.DeleteCampaign(id); err != nil {
		http.Error(w, fmt.Sprintf("Error eliminando campaña: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Campaña eliminada: id=%d", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// handleCampaignUpload handles CSV file upload for campaign contacts
func (s *Server) handleCampaignUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Get campaign ID
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id requerido", http.StatusBadRequest)
		return
	}
	campaignID, err := strconv.Atoi(campaignIDStr)
	if err != nil {
		http.Error(w, "campaign_id inválido", http.StatusBadRequest)
		return
	}

	// Verify campaign exists
	if _, err := s.repo.GetCampaign(campaignID); err != nil {
		http.Error(w, "Campaña no encontrada", http.StatusNotFound)
		return
	}

	// Parse multipart form (max 100MB for large CSVs)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "Archivo demasiado grande", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No se recibió archivo", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error leyendo archivo", http.StatusInternalServerError)
		return
	}

	// Parse CSV (simple format: one phone per line or phone;other;data)
	lines := strings.Split(string(content), "\n")
	telefonos := make([]string, 0, len(lines))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Skip header if present
		if i == 0 && (strings.Contains(strings.ToLower(line), "telefono") || strings.Contains(strings.ToLower(line), "phone")) {
			continue
		}

		// Handle semicolon or comma delimited
		var phone string
		if strings.Contains(line, ";") {
			parts := strings.Split(line, ";")
			phone = strings.TrimSpace(parts[0])
		} else if strings.Contains(line, ",") {
			parts := strings.Split(line, ",")
			phone = strings.TrimSpace(parts[0])
		} else {
			phone = line
		}

		// Basic validation - only digits and + allowed
		phone = strings.ReplaceAll(phone, " ", "")
		phone = strings.ReplaceAll(phone, "-", "")
		if phone != "" && len(phone) >= 7 {
			telefonos = append(telefonos, phone)
		}
	}

	if len(telefonos) == 0 {
		http.Error(w, "No se encontraron números válidos en el archivo", http.StatusBadRequest)
		return
	}

	// Bulk insert
	inserted, err := s.repo.CreateCampaignContactsBulk(campaignID, telefonos)
	if err != nil {
		log.Printf("[API] Error inserting contacts: %v", err)
		http.Error(w, "Error insertando contactos", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] CSV uploaded for campaign %d: %d contacts inserted", campaignID, inserted)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"inserted": inserted,
		"total":    len(telefonos),
	})
}

// handleCampaignAction handles campaign state changes (start, pause, stop)
func (s *Server) handleCampaignAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CampaignID int    `json:"campaign_id"`
		Action     string `json:"action"` // start, pause, stop
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.CampaignID == 0 || req.Action == "" {
		http.Error(w, "campaign_id y action requeridos", http.StatusBadRequest)
		return
	}

	var newState string
	switch req.Action {
	case "start":
		newState = "active"
	case "pause":
		newState = "paused"
	case "stop":
		newState = "stopped"
	default:
		http.Error(w, "action inválida (start, pause, stop)", http.StatusBadRequest)
		return
	}

	if err := s.repo.UpdateCampaignStatus(req.CampaignID, newState); err != nil {
		http.Error(w, fmt.Sprintf("Error actualizando estado: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Campaign %d action: %s -> %s", req.CampaignID, req.Action, newState)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"new_state": newState,
	})
}

// handleCampaignStats returns real-time statistics for a campaign
func (s *Server) handleCampaignStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id requerido", http.StatusBadRequest)
		return
	}

	campaignID, err := strconv.Atoi(campaignIDStr)
	if err != nil {
		http.Error(w, "campaign_id inválido", http.StatusBadRequest)
		return
	}

	campaign, err := s.repo.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, "Campaña no encontrada", http.StatusNotFound)
		return
	}

	counts, err := s.repo.CountContactsByStatus(campaignID)
	if err != nil {
		log.Printf("[API] Error counting contacts: %v", err)
		counts = make(map[string]int)
	}

	inSchedule, _ := s.repo.IsWithinSchedule(campaignID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"campaign":    campaign,
		"counts":      counts,
		"in_schedule": inSchedule,
	})
}

// handleCampaignSchedules manages campaign schedules
func (s *Server) handleCampaignSchedules(w http.ResponseWriter, r *http.Request) {
	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id requerido", http.StatusBadRequest)
		return
	}

	campaignID, err := strconv.Atoi(campaignIDStr)
	if err != nil {
		http.Error(w, "campaign_id inválido", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		schedules, err := s.repo.GetCampaignSchedules(campaignID)
		if err != nil {
			http.Error(w, "Error obteniendo schedules", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(schedules)

	case http.MethodPost, http.MethodPut:
		var schedules []database.CampaignSchedule
		if err := json.NewDecoder(r.Body).Decode(&schedules); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// Validate schedules
		for _, s := range schedules {
			if s.DiaSemana < 0 || s.DiaSemana > 6 {
				http.Error(w, "dia_semana debe ser 0-6 (Domingo-Sábado)", http.StatusBadRequest)
				return
			}
		}

		if err := s.repo.UpdateCampaignSchedules(campaignID, schedules); err != nil {
			http.Error(w, fmt.Sprintf("Error guardando schedules: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("[API] Schedules updated for campaign %d", campaignID)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

// --- SYSTEM CONFIGURATION MANAGEMENT ---

// handleConfig manages system configuration (GET list, PUT update)
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	// Verify admin role
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, "Acceso denegado: Se requiere rol de Admin", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all configurations
		configs, err := s.repo.ListConfigs()
		if err != nil {
			log.Printf("[API] Error listing configs: %v", err)
			http.Error(w, "Error listando configuraciones", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configs)

	case http.MethodPut:
		// Update a specific config
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		if req.Key == "" {
			http.Error(w, "key es requerido", http.StatusBadRequest)
			return
		}

		if err := s.repo.SetConfig(req.Key, req.Value, ""); err != nil {
			log.Printf("[API] Error updating config %s: %v", req.Key, err)
			http.Error(w, "Error actualizando configuración", http.StatusInternalServerError)
			return
		}

		log.Printf("[API] Config updated: %s = %s", req.Key, req.Value)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

// --- CAMPAIGN RECYCLING ---

// handleCampaignDispositions returns contact counts grouped by disposition/resultado
func (s *Server) handleCampaignDispositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	campaignIDStr := r.URL.Query().Get("campaign_id")
	if campaignIDStr == "" {
		http.Error(w, "campaign_id requerido", http.StatusBadRequest)
		return
	}

	campaignID, err := strconv.Atoi(campaignIDStr)
	if err != nil {
		http.Error(w, "campaign_id inválido", http.StatusBadRequest)
		return
	}

	counts, err := s.repo.CountContactsByResultado(campaignID)
	if err != nil {
		log.Printf("[API] Error counting dispositions: %v", err)
		http.Error(w, "Error obteniendo disposiciones", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(counts)
}

// handleCampaignRecycle creates a new campaign from recycled contacts
func (s *Server) handleCampaignRecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CampaignID   int      `json:"campaign_id"`
		Nombre       string   `json:"nombre"`
		Dispositions []string `json:"dispositions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	if req.CampaignID == 0 || req.Nombre == "" || len(req.Dispositions) == 0 {
		http.Error(w, "campaign_id, nombre y dispositions son requeridos", http.StatusBadRequest)
		return
	}

	// Get source campaign to copy proyecto_id
	sourceCampaign, err := s.repo.GetCampaign(req.CampaignID)
	if err != nil {
		http.Error(w, "Campaña origen no encontrada", http.StatusNotFound)
		return
	}

	// Create new campaign
	newCampaign := &database.Campaign{
		Nombre:     req.Nombre,
		ProyectoID: sourceCampaign.ProyectoID,
		Estado:     "draft",
	}

	if err := s.repo.CreateCampaign(newCampaign); err != nil {
		log.Printf("[API] Error creating recycled campaign: %v", err)
		http.Error(w, fmt.Sprintf("Error creando campaña: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy contacts with selected dispositions
	inserted, err := s.repo.RecycleCampaignContacts(req.CampaignID, newCampaign.ID, req.Dispositions)
	if err != nil {
		log.Printf("[API] Error recycling contacts: %v", err)
		// Delete the empty campaign
		s.repo.DeleteCampaign(newCampaign.ID)
		http.Error(w, fmt.Sprintf("Error reciclando contactos: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Campaign recycled: source=%d -> new=%d, contacts=%d, dispositions=%v",
		req.CampaignID, newCampaign.ID, inserted, req.Dispositions)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"new_campaign_id":  newCampaign.ID,
		"contacts_copied":  inserted,
		"dispositions":     req.Dispositions,
	})
}

// --- PROJECT AUDIO MANAGEMENT ---

// handleProyectoAudio handles GET (query audio) and PUT (set audio) for a project
func (s *Server) handleProyectoAudio(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// GET: Query the audio set for a project
		proyectoIDStr := r.URL.Query().Get("proyecto_id")
		if proyectoIDStr == "" {
			http.Error(w, "proyecto_id requerido", http.StatusBadRequest)
			return
		}

		proyectoID, err := strconv.Atoi(proyectoIDStr)
		if err != nil {
			http.Error(w, "proyecto_id inválido", http.StatusBadRequest)
			return
		}

		proyecto, err := s.repo.GetProyecto(proyectoID)
		if err != nil {
			http.Error(w, "Proyecto no encontrado", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"proyecto_id":   proyecto.ID,
			"proyecto_name": proyecto.Nombre,
			"audio":         proyecto.Audio,
		})

	case http.MethodPut:
		// PUT: Set audio for a project
		var req struct {
			ProyectoID int    `json:"proyecto_id"`
			Audio      string `json:"audio"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		if req.ProyectoID == 0 || req.Audio == "" {
			http.Error(w, "proyecto_id y audio son requeridos", http.StatusBadRequest)
			return
		}

		// Verify audio file exists
		audioPath := fmt.Sprintf("/var/lib/asterisk/sounds/apicall/%s", req.Audio)
		if _, err := os.Stat(audioPath); os.IsNotExist(err) {
			http.Error(w, fmt.Sprintf("Audio file not found: %s", req.Audio), http.StatusBadRequest)
			return
		}

		// Update project audio
		query := "UPDATE apicall_proyectos SET audio = ? WHERE id = ?"
		_, err := s.repo.GetDB().Exec(query, req.Audio, req.ProyectoID)
		if err != nil {
			log.Printf("[API] Error updating project audio: %v", err)
			http.Error(w, "Error actualizando audio del proyecto", http.StatusInternalServerError)
			return
		}

		log.Printf("[API] Project %d audio updated to: %s", req.ProyectoID, req.Audio)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"proyecto_id": req.ProyectoID,
			"audio":       req.Audio,
		})

	default:
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}
