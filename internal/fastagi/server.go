package fastagi

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"apicall/internal/config"
	"apicall/internal/database"
)

// Server representa el servidor FastAGI
type Server struct {
	config *config.Config
	repo   *database.Repository
	mu     sync.Mutex
	active map[string]*Session // Sesiones activas por uniqueid
}

// NewServer crea un nuevo servidor FastAGI
func NewServer(cfg *config.Config, repo *database.Repository) *Server {
	return &Server{
		config: cfg,
		repo:   repo,
		active: make(map[string]*Session),
	}
}

// Start inicia el servidor FastAGI
func (s *Server) Start() error {
	addr := s.config.FastAGI.Address()
	log.Printf("[FastAGI] Iniciando servidor en %s", addr)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("error iniciando listener: %w", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("[FastAGI] Error aceptando conexión: %v", err)
				continue
			}

			go s.handleConnection(conn)
		}
	}()

	log.Printf("[FastAGI] Servidor iniciado correctamente")
	return nil
}

// handleConnection maneja una conexión AGI entrante
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Protección contra Pánicos (Panic Recovery)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FastAGI] PANIC RECOVERED: %v", r)
		}
	}()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Parsear variables AGI iniciales
	vars, err := parseAGIVariables(reader)
	if err != nil {
		log.Printf("[FastAGI] Error parseando variables: %v", err)
		return
	}

	// Crear sesión
	session := NewSession(conn, reader, writer, vars, s.config, s.repo)

	// Registrar sesión activa
	uniqueid := vars["agi_uniqueid"]
	s.mu.Lock()
	s.active[uniqueid] = session
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.active, uniqueid)
		s.mu.Unlock()
	}()

	log.Printf("[FastAGI] Nueva sesión: %s desde %s", uniqueid, vars["agi_callerid"])

	// Ejecutar lógica de IVR
	if err := session.HandleIVR(); err != nil {
		log.Printf("[FastAGI] Error en IVR: %v", err)
	}
}

// parseAGIVariables lee las variables iniciales del protocolo AGI
func parseAGIVariables(reader *bufio.Reader) (map[string]string, error) {
	vars := make(map[string]string)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // Fin de las variables
		}

		// Formato: agi_variable: value
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			vars[parts[0]] = parts[1]
		}
	}

	return vars, nil
}

// GetActiveSessionCount devuelve el número de sesiones activas
func (s *Server) GetActiveSessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active)
}
