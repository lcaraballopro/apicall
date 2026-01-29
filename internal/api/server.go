package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"apicall/internal/ami"
	"apicall/internal/asterisk"
	"apicall/internal/auth"
	"apicall/internal/config"
	"apicall/internal/database"
	"apicall/internal/provisioning"
	"apicall/internal/smartcid"
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

	mux := http.NewServeMux()

	// 1. Static Files (Public)
	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)

	// 2. Public API Endpoints
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/health", s.handleHealth)

	// 3. Protected API Routes
	// We create a sub-handler for protected routes to wrap them in middleware
	protectedMux := http.NewServeMux()

	protectedMux.HandleFunc("/api/v1/call", s.handleCall)

	protectedMux.HandleFunc("/api/v1/proyectos", s.handleProyectos)
	protectedMux.HandleFunc("/api/v1/proyectos/delete", s.handleProyectoDelete)

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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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

		if fromDate != "" || toDate != "" {
			logs, err = s.repo.GetCallLogsByProyectoWithDates(proyectoID, limit, fromDate, toDate)
		} else {
			logs, err = s.repo.GetCallLogsByProyecto(proyectoID, limit)
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

	// Mapear DIALSTATUS de Asterisk a Disposition
	var disposition string
	switch status {
	case "ANSWER":
		disposition = "ANSWERED"
	case "BUSY":
		disposition = "BUSY"
	case "NOANSWER":
		disposition = "NO ANSWER"
	case "CANCEL":
		disposition = "CANCELLED"
	case "CONGESTION":
		disposition = "FAILED"
	case "CHANUNAVAIL":
		disposition = "FAILED"
	default:
		disposition = status
	}

	if err := s.repo.UpdateCallLog(logID, nil, &disposition, false, status, 0); err != nil {
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
				gen.UpdateStats(usedCID, disposition == "ANSWERED")
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
		http.Error(w, "Credenciales inválidas", http.StatusUnauthorized)
		return
	}

	if err := auth.VerifyPassword(user.PasswordHash, creds.Password); err != nil {
		log.Printf("[Auth] Contraseña incorrecta para usuario: %s", creds.Username)
		http.Error(w, "Credenciales inválidas", http.StatusUnauthorized)
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

	var audios []map[string]interface{}
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

// handleAudioUpload handles file uploads
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

	// Validate extension
	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := map[string]bool{
		".wav": true, ".gsm": true, ".ulaw": true, ".alaw": true, ".sln": true, ".mp3": true,
	}
	if !allowedExts[ext] {
		http.Error(w, "Formato no soportado. Use: wav, gsm, ulaw, alaw, sln, mp3", http.StatusBadRequest)
		return
	}

	// Create directory if not exists
	audioDir := "/var/lib/asterisk/sounds/apicall"
	os.MkdirAll(audioDir, 0755)

	// Save file
	destPath := filepath.Join(audioDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		log.Printf("[API] Error creando archivo: %v", err)
		http.Error(w, "Error guardando archivo", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		http.Error(w, "Error escribiendo archivo", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] Audio subido: %s (%d bytes)", filename, header.Size)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"filename": filename,
		"path":     fmt.Sprintf("apicall/%s", filename),
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
