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
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	vars       map[string]string
	config     *config.Config
	repo       *database.Repository
	logID      int64 // ID del registro en apicall_call_log
	contactID  int64 // ID del contacto de campaña (0 si no aplica)
	campaignID int   // ID de la campaña (0 si no aplica)
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

	// Obtener ID del proyecto desde argumentos AGI o Variables de Canal
	proyectoIDStr := s.vars["agi_arg_1"]
	if proyectoIDStr == "" {
		// Fallback: Check for APICALL_PROJECT_ID channel variable (set by AMIDialer)
		var err error
		proyectoIDStr, err = s.GetVariable("APICALL_PROJECT_ID")
		if err != nil || proyectoIDStr == "" {
			s.Verbose("Apicall Error: No se recibio proyecto_id (arg1) ni APICALL_PROJECT_ID", 3)
			return fmt.Errorf("no se proporcionó proyecto_id")
		}
		s.Verbose("Apicall: Proyecto ID recuperado de variable APICALL_PROJECT_ID", 3)
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
		
		// Obtener IDs de contacto y campaña para correlación
		contactIDStr, _ := s.GetVariable("APICALL_CONTACT_ID")
		if contactIDStr != "" {
			s.contactID, _ = strconv.ParseInt(contactIDStr, 10, 64)
		}
		campaignIDStr, _ := s.GetVariable("APICALL_CAMPAIGN_ID")
		if campaignIDStr != "" {
			campID, _ := strconv.Atoi(campaignIDStr)
			s.campaignID = campID
		}
		if s.contactID > 0 {
			s.Verbose(fmt.Sprintf("Apicall: Correlacion campaign=%d contact=%d", s.campaignID, s.contactID), 3)
		}
		
		// Actualizar a Connected y guardar uniqueid de Asterisk
		uniqueid := s.vars["agi_uniqueid"]
		s.updateLog("CONNECTED", "A", false, "", 0, &uniqueid)
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

		// Obtener campaign_id si existe
		var campaignID *int
		campaignIDStr, _ := s.GetVariable("APICALL_CAMPAIGN_ID")
		if campaignIDStr != "" && campaignIDStr != "0" {
			cid, _ := strconv.Atoi(campaignIDStr)
			if cid > 0 {
				campaignID = &cid
				s.campaignID = cid
			}
		}

		// Obtener contact_id si existe
		contactIDStr, _ := s.GetVariable("APICALL_CONTACT_ID")
		if contactIDStr != "" {
			s.contactID, _ = strconv.ParseInt(contactIDStr, 10, 64)
		}

		// Obtener caller_id usado (el efectivo del canal)
		callerIDUsed := s.vars["agi_callerid"]
		if callerIDUsed == "" {
			callerIDUsed = proyecto.CallerID // Fallback al proyecto
		}

		callLog := &database.CallLog{
			ProyectoID:   proyectoID,
			Telefono:     telefonoDestino,
			Interacciono: false,
			Status:       "INITIATED_LEGACY",
			Uniqueid:     uniqueid,
			CampaignID:   campaignID,
			CallerIDUsed: callerIDUsed,
		}

		log.Printf("[Session] DEBUG: Creating CallLog with uniqueid='%s', telefono='%s', campaign_id=%v, caller_id='%s'", 
			uniqueid, telefonoDestino, campaignID, callerIDUsed)
		logID, err := s.repo.CreateCallLog(callLog)
		if err != nil {
			log.Printf("[Session] Warning: error creando log: %v", err)
		}
		s.logID = logID
	}

	// Responder la llamada
	log.Printf("[Session] DEBUG: Antes de Answer() - Proyecto %d", proyecto.ID)
	s.Verbose("Apicall: Respondiendo llamada...", 3)
	if err := s.Answer(); err != nil {
		log.Printf("[Session] ERROR: Answer() falló: %v", err)
		s.updateLog("COMPLETED", "NA", false, "", int(time.Since(startTime).Seconds()), nil)
		return err
	}
	log.Printf("[Session] DEBUG: Answer() exitoso")

	// Verificar si AMD está activo
	if proyecto.AMDActive {
		s.Verbose("Apicall: Ejecutando AMD (Answering Machine Detection)...", 3)
		// Parámetros AMD ultra-rápidos para detección inmediata:
		// initial_silence=1500ms (antes 2500), greeting=1000ms (antes 1500), 
		// after_greeting_silence=500ms (antes 1000), total_analysis_time=3000ms (antes 5000), 
		// min_word_length=100, between_words_silence=50, maximum_number_of_words=3, silence_threshold=256
		amdParams := "1500|1000|500|3000|100|50|3|256"
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
				s.updateLog("COMPLETED", "AM", true, "", int(time.Since(startTime).Seconds()), nil)
				return s.Hangup()
			} else if amdStatus == "HUMAN" {
				s.Verbose("Apicall: Humano detectado. Continuando.", 3)
				// CRITICAL: Update status immediately so we don't lose the "Answered" state if they hangup during audio
				s.updateLog("HUMAN", "A", true, "", int(time.Since(startTime).Seconds()), nil)
			} else {
				s.Verbose(fmt.Sprintf("Apicall: AMD Incierto (%s). Asumiendo humano.", amdStatus), 3)
				// Treat uncertain as human (Answered)
				s.updateLog("HUMAN", "A", true, "", int(time.Since(startTime).Seconds()), nil)
			}
		}
	}

	// Reproducir audio principal
	audioPath := fmt.Sprintf("%s/%s", s.config.Asterisk.SoundPath, proyecto.Audio)
	log.Printf("[Session] DEBUG: Antes de StreamFile() - Path: %s", audioPath)
	s.Verbose(fmt.Sprintf("Apicall: Reproduciendo archivo '%s'...", audioPath), 3)
	
	if err := s.StreamFile(audioPath); err != nil {
		log.Printf("[Session] ERROR: StreamFile() falló: %v", err)
		s.Verbose(fmt.Sprintf("Apicall Error: Fallo reproduccion: %v", err), 3)
		s.updateLog("COMPLETED", "FAIL", true, "", int(time.Since(startTime).Seconds()), nil)
		return err
	}
	log.Printf("[Session] DEBUG: StreamFile() exitoso")

	// Lógica de reintentos para DTMF
	maxAttempts := 2
	invalidAudio := fmt.Sprintf("%s/opcion_invalida", s.config.Asterisk.SoundPath)
	confirmAudio := fmt.Sprintf("%s/en_breve", s.config.Asterisk.SoundPath)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		s.Verbose(fmt.Sprintf("Apicall: Esperando DTMF (Intento %d/%d, Timeout 10s)...", attempt, maxAttempts), 3)
		
		dtmf, err := s.WaitForDTMF(10) // 10 segundos timeout
		
		if err != nil {
			// Timeout - no se recibió ningún DTMF
			s.Verbose(fmt.Sprintf("Apicall: Timeout esperando DTMF (Intento %d)", attempt), 3)
			
			if attempt < maxAttempts {
				// Reproducir audio de opción inválida y reintentar
				s.StreamFile(invalidAudio)
				continue
			} else {
				// Segundo intento fallido, colgar
				s.Verbose("Apicall: Sin respuesta tras 2 intentos. Terminando.", 3)
				s.updateLog("COMPLETED", "N", true, "", int(time.Since(startTime).Seconds()), nil)
				return nil
			}
		}

		log.Printf("[Session] DTMF recibido: %s (esperado: %s)", dtmf, proyecto.DTMFEsperado)
		s.Verbose(fmt.Sprintf("Apicall: DTMF Recibido: '%s' (Esperado: '%s')", dtmf, proyecto.DTMFEsperado), 3)

		// Verificar si el DTMF es el esperado
		if dtmf == proyecto.DTMFEsperado {
			// DTMF correcto - reproducir confirmación y transferir
			s.Verbose(fmt.Sprintf("Apicall: DTMF correcto. Reproduciendo confirmacion..."), 3)
			s.StreamFile(confirmAudio)
			
			s.Verbose(fmt.Sprintf("Apicall: Transfiriendo a %s...", proyecto.NumeroDesborde), 3)
			if err := s.Transfer(proyecto); err != nil {
				s.updateLog("FAILED", "FAIL", true, dtmf, int(time.Since(startTime).Seconds()), nil)
				return err
			}
			s.updateLog("COMPLETED", "XFER", true, dtmf, int(time.Since(startTime).Seconds()), nil)
			s.Verbose("=== Apicall: Sesion Terminada ===", 3)
			return nil
		} else {
			// DTMF incorrecto
			s.Verbose(fmt.Sprintf("Apicall: DTMF incorrecto '%s'", dtmf), 3)
			
			if attempt < maxAttempts {
				// Reproducir audio de opción inválida y reintentar
				s.StreamFile(invalidAudio)
				continue
			} else {
				// Segundo intento con DTMF incorrecto, colgar
				s.Verbose("Apicall: DTMF incorrecto tras 2 intentos. Terminando.", 3)
				s.updateLog("COMPLETED", "N", true, dtmf, int(time.Since(startTime).Seconds()), nil)
				return nil
			}
		}
	}
	
	s.Verbose("=== Apicall: Sesion Terminada ===", 3)
	return nil
}

// Transfer transfiere la llamada al número de desborde
func (s *Session) Transfer(proyecto *database.Proyecto) error {
	log.Printf("[Session] Transfiriendo a %s vía %s", proyecto.NumeroDesborde, proyecto.TroncalSalida)

	// Establecer variables de canal para que el dialplan ejecute la transferencia
	s.SetVariable("APICALL_TRUNK", proyecto.TroncalSalida)
	s.SetVariable("APICALL_PREFIX", proyecto.PrefijoSalida)
	s.SetVariable("APICALL_CALLERID", proyecto.CallerID)
	s.SetVariable("APICALL_TRANSFER", proyecto.NumeroDesborde)

	// El dialplan revisará APICALL_TRANSFER después del AGI y ejecutará el Dial
	return nil
}

// updateLog actualiza el registro de llamada y el estado del contacto si aplica
func (s *Session) updateLog(status string, disposition string, interacciono bool, dtmf string, duracion int, uniqueid *string) {
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

	if err := s.repo.UpdateCallLog(s.logID, dtmfPtr, dispositionPtr, uniqueid, interacciono, status, duracion); err != nil {
		log.Printf("[Session] Error actualizando log: %v", err)
	}

	// Actualizar estado del contacto de campaña si aplica
	if s.contactID > 0 {
		contactStatus := mapCallStatusToContactStatus(status)
		if err := s.repo.UpdateContactStatus(s.contactID, contactStatus, &status); err != nil {
			log.Printf("[Session] Error actualizando contacto %d: %v", s.contactID, err)
		} else {
			log.Printf("[Session] Contacto %d actualizado a '%s' (call status: %s)", s.contactID, contactStatus, status)
		}
	}
}

// mapCallStatusToContactStatus convierte la disposition de llamada al estado del contacto
func mapCallStatusToContactStatus(disposition string) string {
	switch disposition {
	case "XFER", "A": // Transferred or Answered
		return "completed"
	case "AM", "NA", "N", "B", "FAIL", "CONG", "NI", "DNC":
		return "failed"
	default:
		return "completed" // Fallback
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
