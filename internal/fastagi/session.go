package fastagi

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"apicall/internal/config"
	"apicall/internal/database"
)

// Session representa una sesión AGI individual
type Session struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	vars   map[string]string
	config *config.Config
	repo   *database.Repository
	logID  int64 // ID del registro en apicall_call_log
}

// NewSession crea una nueva sesión AGI
func NewSession(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer,
	vars map[string]string, cfg *config.Config, repo *database.Repository) *Session {
	return &Session{
		conn:   conn,
		reader: reader,
		writer: writer,
		vars:   vars,
		config: cfg,
		repo:   repo,
	}
}

// HandleIVR ejecuta la lógica principal del IVR
func (s *Session) HandleIVR() error {
	startTime := time.Now()

	// Anunciar inicio en CLI Asterisk
	s.Verbose("=== Apicall: Nueva Sesion ===", 3)

	// Obtener ID del proyecto desde variable de canal
	proyectoIDStr := s.vars["agi_arg_1"]
	if proyectoIDStr == "" {
		s.Verbose("Apicall Error: No se recibio proyecto_id", 3)
		return fmt.Errorf("no se proporcionó proyecto_id")
	}

	s.Verbose(fmt.Sprintf("Apicall: Proyecto ID recibido: %s", proyectoIDStr), 3)

	proyectoID, err := strconv.Atoi(proyectoIDStr)
	if err != nil {
		s.Verbose(fmt.Sprintf("Apicall Error: ID invalido: %v", err), 3)
		return fmt.Errorf("proyecto_id inválido: %w", err)
	}

	// Obtener configuración del proyecto
	proyecto, err := s.repo.GetProyecto(proyectoID)
	if err != nil {
		s.Verbose(fmt.Sprintf("Apicall Error: Proyecto no encontrado en DB: %v", err), 3)
		return fmt.Errorf("error obteniendo proyecto: %w", err)
	}

	log.Printf("[Session] Proyecto: %s (#%d)", proyecto.Nombre, proyecto.ID)
	s.Verbose(fmt.Sprintf("Apicall: Cargado Proyecto '%s' (Audio: %s)", proyecto.Nombre, proyecto.Audio), 3)

	// Intentar obtener ID de log pre-creado (Spooler)
	logIDStr, _ := s.GetVariable("APICALL_LOG_ID")
	if logIDStr != "" {
		s.logID, _ = strconv.ParseInt(logIDStr, 10, 64)
		s.Verbose(fmt.Sprintf("Apicall: Usando Log pre-creado ID %d", s.logID), 3)
		// Actualizar a Answered
		s.updateLog("ANSWERED", "ANSWERED", false, "", 0)
	} else {
		// Fallback: Crear log si no existe (ej. llamada directa)
		uniqueid := s.vars["agi_uniqueid"]
		
		// Obtener el teléfono destino de la variable de canal
		telefonoDestino, err := s.GetVariable("APICALL_TELEFONO")
		if err != nil || telefonoDestino == "" {
			// Fallback si no existe la variable (ej. llamadas directas)
			telefonoDestino = s.vars["agi_callerid"]
			s.Verbose("Apicall Warning: No se encontró APICALL_TELEFONO, usando CallerID", 3)
		}

		callLog := &database.CallLog{
			ProyectoID:   proyectoID,
			Telefono:     telefonoDestino,
			Interacciono: false,
			Status:       "INITIATED_LEGACY",
			Uniqueid:     uniqueid,
		}

		logID, err := s.repo.CreateCallLog(callLog)
		if err != nil {
			log.Printf("[Session] Warning: error creando log: %v", err)
		}
		s.logID = logID
	}

	// Responder la llamada
	s.Verbose("Apicall: Respondiendo llamada...", 3)
	// Responder la llamada
	s.Verbose("Apicall: Respondiendo llamada...", 3)
	if err := s.Answer(); err != nil {
		s.updateLog("NO-ANSWER", "NO ANSWER", false, "", int(time.Since(startTime).Seconds()))
		return err
	}

	// Verificar si AMD está activo
	if proyecto.AMDActive {
		s.Verbose("Apicall: Ejecutando AMD (Answering Machine Detection)...", 3)
		// Parámetros AMD optimizados para menor latencia:
		// initial_silence=2500, greeting=1500, after_greeting_silence=800, total_analysis_time=5000, 
		// min_word_length=100, between_words_silence=50, maximum_number_of_words=5, silence_threshold=256
		amdParams := "2500|1500|1000|5000|100|50|4|256"
		if err := s.Exec("AMD", amdParams); err != nil {
			s.Verbose(fmt.Sprintf("Apicall Warning: Error ejecutando AMD: %v", err), 3)
		} else {
			// Obtener resultado
			amdStatus, _ := s.GetVariable("AMDSTATUS")
			amdCause, _ := s.GetVariable("AMDCAUSE")
			s.Verbose(fmt.Sprintf("Apicall: AMD Resultado: %s (Causa: %s)", amdStatus, amdCause), 3)

			if amdStatus == "MACHINE" {
				// Es máquina, colgar
				s.Verbose("Apicall: Maquina detectada. Colgando.", 3)
				s.updateLog("AMD_MACHINE", "AMD_MACHINE", true, "", int(time.Since(startTime).Seconds()))
				return s.Hangup()
			} else if amdStatus == "HUMAN" {
				s.Verbose("Apicall: Humano detectado. Continuando.", 3)
			} else {
				s.Verbose(fmt.Sprintf("Apicall: AMD Incierto (%s). Asumiendo humano.", amdStatus), 3)
			}
		}
	}

	// Reproducir audio
	audioPath := fmt.Sprintf("%s/%s", s.config.Asterisk.SoundPath, proyecto.Audio)
	s.Verbose(fmt.Sprintf("Apicall: Reproduciendo archivo '%s'...", audioPath), 3)
	
	if err := s.StreamFile(audioPath); err != nil {
		s.Verbose(fmt.Sprintf("Apicall Error: Fallo reproduccion: %v", err), 3)
		s.updateLog("AUDIO-ERROR", "ANSWERED", true, "", int(time.Since(startTime).Seconds()))
		return err
	}

	// Esperar DTMF
	s.Verbose(fmt.Sprintf("Apicall: Esperando DTMF (Timeout 10s)..."), 3)
	dtmf, err := s.WaitForDTMF(10) // 10 segundos timeout
	if err != nil {
		s.Verbose("Apicall: Timeout esperando DTMF", 3)
		s.updateLog("TIMEOUT", "ANSWERED", true, "", int(time.Since(startTime).Seconds()))
		return nil
	}

	log.Printf("[Session] DTMF recibido: %s (esperado: %s)", dtmf, proyecto.DTMFEsperado)
	s.Verbose(fmt.Sprintf("Apicall: DTMF Recibido: '%s' (Esperado: '%s')", dtmf, proyecto.DTMFEsperado), 3)

	// Verificar si el DTMF es el esperado
	if dtmf == proyecto.DTMFEsperado {
		// Transferir llamada
		s.Verbose(fmt.Sprintf("Apicall: DTMF correcto. Transfiriendo a %s...", proyecto.NumeroDesborde), 3)
		if err := s.Transfer(proyecto); err != nil {
			s.updateLog("TRANSFER-ERROR", "FAILED", true, dtmf, int(time.Since(startTime).Seconds()))
			return err
		}
		s.updateLog("TRANSFERRED", "TRANSFERRED", true, dtmf, int(time.Since(startTime).Seconds()))
	} else {
		// DTMF incorrecto
		s.Verbose(fmt.Sprintf("Apicall: DTMF incorrecto. Terminando."), 3)
		s.updateLog("INVALID-DTMF", "ANSWERED", true, dtmf, int(time.Since(startTime).Seconds()))
	}
	
	s.Verbose("=== Apicall: Sesion Terminada ===", 3)
	return nil
}

// Transfer transfiere la llamada al número de desborde
func (s *Session) Transfer(proyecto *database.Proyecto) error {
	log.Printf("[Session] Transfiriendo a %s vía %s", proyecto.NumeroDesborde, proyecto.TroncalSalida)

	// Establecer variables de canal para el contexto de salida
	s.SetVariable("APICALL_TRUNK", proyecto.TroncalSalida)
	s.SetVariable("APICALL_PREFIX", proyecto.PrefijoSalida)
	s.SetVariable("APICALL_CALLERID", proyecto.CallerID)

	// Transferir al contexto de salida
	return s.Exec("Goto", fmt.Sprintf("%s,%s,1", s.config.Asterisk.OutboundContext, proyecto.NumeroDesborde))
}

// updateLog actualiza el registro de llamada
func (s *Session) updateLog(status string, disposition string, interacciono bool, dtmf string, duracion int) {
	if s.logID == 0 {
		return
	}

	var dtmfPtr *string
	if dtmf != "" {
		dtmfPtr = &dtmf
	}

	var dispositionPtr *string
	if disposition != "" {
		dispositionPtr = &disposition
	}

	if err := s.repo.UpdateCallLog(s.logID, dtmfPtr, dispositionPtr, interacciono, status, duracion); err != nil {
		log.Printf("[Session] Error actualizando log: %v", err)
	}
}

// ===== Comandos AGI =====

// execCommand ejecuta un comando AGI y devuelve la respuesta
func (s *Session) execCommand(cmd string) (string, error) {
	// Enviar comando
	if _, err := s.writer.WriteString(cmd + "\n"); err != nil {
		return "", err
	}
	if err := s.writer.Flush(); err != nil {
		return "", err
	}

	// Leer respuesta
	response, err := s.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	response = strings.TrimSpace(response)

	// Verificar error
	if strings.HasPrefix(response, "520") {
		return "", fmt.Errorf("comando inválido: %s", cmd)
	}

	return response, nil
}

// GetVariable obtiene el valor de una variable de canal
func (s *Session) GetVariable(name string) (string, error) {
	resp, err := s.execCommand(fmt.Sprintf("GET VARIABLE %s", name))
	if err != nil {
		return "", err
	}

	// Parsear respuesta: 200 result=1 (<value>)
	// Ejemplo: 200 result=1 (5551234)
	parts := strings.SplitN(resp, "(", 2)
	if len(parts) < 2 {
		return "", nil // Variable no set o vacía
	}
	
	value := strings.TrimSuffix(parts[1], ")")
	return value, nil
}

// Answer responde la llamada
func (s *Session) Answer() error {
	_, err := s.execCommand("ANSWER")
	return err
}

// StreamFile reproduce un archivo de audio
func (s *Session) StreamFile(file string) error {
	// Remover extensión si existe
	file = strings.TrimSuffix(file, ".wav")
	file = strings.TrimSuffix(file, ".gsm")

	_, err := s.execCommand(fmt.Sprintf("STREAM FILE %s \"\"", file))
	return err
}

// WaitForDTMF espera un dígito DTMF con timeout
func (s *Session) WaitForDTMF(timeout int) (string, error) {
	resp, err := s.execCommand(fmt.Sprintf("WAIT FOR DIGIT %d", timeout*1000))
	if err != nil {
		return "", err
	}

	// Parsear respuesta: 200 result=<digit>
	// Ejemplo: 200 result=49 (código ASCII del '1')
	// Ejemplo: 200 result=0 (timeout)
	parts := strings.Split(resp, "=")
	if len(parts) < 2 {
		return "", fmt.Errorf("respuesta inválida: %s", resp)
	}

	digitStr := strings.TrimSpace(parts[1])
	digitCode, err := strconv.Atoi(digitStr)
	if err != nil {
		return "", fmt.Errorf("código DTMF inválido: %s", digitStr)
	}

	if digitCode == 0 {
		return "", fmt.Errorf("timeout esperando DTMF")
	}

	// Validar rango ASCII para 0-9, *, #
	// 0-9: 48-57
	// *: 42
	// #: 35
	if (digitCode >= 48 && digitCode <= 57) || digitCode == 42 || digitCode == 35 {
		return string(rune(digitCode)), nil
	}

	// Si recibimos algo fuera de rango, lo ignoramos o retornamos error
	return "", fmt.Errorf("DTMF inválido (ASCII %d)", digitCode)
}

// SetVariable establece una variable de canal
func (s *Session) SetVariable(name, value string) error {
	_, err := s.execCommand(fmt.Sprintf("SET VARIABLE %s \"%s\"", name, value))
	return err
}

// Exec ejecuta una aplicación de Asterisk
func (s *Session) Exec(app string, args string) error {
	_, err := s.execCommand(fmt.Sprintf("EXEC %s %s", app, args))
	return err
}

// Hangup cuelga la llamada
func (s *Session) Hangup() error {
	_, err := s.execCommand("HANGUP")
	return err
}

// Verbose envía un mensaje al CLI de Asterisk
func (s *Session) Verbose(msg string, level int) error {
	_, err := s.execCommand(fmt.Sprintf("VERBOSE \"%s\" %d", msg, level))
	return err
}
