package ami

import (
	"fmt"
	"log"

	"apicall/internal/database"
)

// OriginateParams parámetros para originar una llamada
type OriginateParams struct {
	Channel     string            // Canal de salida (ej: SIP/trunk/numero)
	Context     string            // Contexto de destino
	Extension   string            // Extensión de destino (usualmente 's')
	Priority    int               // Prioridad (usualmente 1)
	CallerID    string            // Caller ID a mostrar
	Timeout     int               // Timeout en milisegundos
	Variables   map[string]string // Variables de canal
	Async       bool              // Si es asíncrono
}

// Originate genera una llamada saliente
func (c *Client) Originate(params OriginateParams) error {
	log.Printf("[AMI] Originando llamada a %s", params.Channel)

	// Construir acción Originate
	action := fmt.Sprintf("Action: Originate\r\n")
	action += fmt.Sprintf("Channel: %s\r\n", params.Channel)
	action += fmt.Sprintf("Context: %s\r\n", params.Context)
	action += fmt.Sprintf("Exten: %s\r\n", params.Extension)
	action += fmt.Sprintf("Priority: %d\r\n", params.Priority)
	action += fmt.Sprintf("CallerID: %s\r\n", params.CallerID)
	action += fmt.Sprintf("Timeout: %d\r\n", params.Timeout)

	if params.Async {
		action += "Async: true\r\n"
	}

	// Agregar variables de canal
	for key, value := range params.Variables {
		action += fmt.Sprintf("Variable: %s=%s\r\n", key, value)
	}

	action += "\r\n"

	// Enviar acción
	return c.sendAction(action)
}

// OriginateCall genera una llamada para un proyecto específico
func (c *Client) OriginateCall(proyecto *database.Proyecto, telefono string) error {
	// Construir canal de salida
	channel := fmt.Sprintf("SIP/%s/%s%s",
		proyecto.TroncalSalida,
		proyecto.PrefijoSalida,
		telefono,
	)

	// Variables de canal
	variables := map[string]string{
		"PROYECTO_ID":       fmt.Sprintf("%d", proyecto.ID),
		"PROYECTO_NOMBRE":   proyecto.Nombre,
		"APICALL_TELEFONO":  telefono,
		"APICALL_TRUNK":     proyecto.TroncalSalida,
		"APICALL_PREFIX":    proyecto.PrefijoSalida,
		"APICALL_CALLERID":  proyecto.CallerID,
	}

	params := OriginateParams{
		Channel:   channel,
		Context:   "apicall_context",
		Extension: "s",
		Priority:  1,
		CallerID:  proyecto.CallerID,
		Timeout:   60000, // 60 segundos
		Variables: variables,
		Async:     true,
	}

	return c.Originate(params)
}

// Hangup cuelga un canal específico
func (c *Client) Hangup(channel string, cause string) error {
	action := fmt.Sprintf("Action: Hangup\r\n")
	action += fmt.Sprintf("Channel: %s\r\n", channel)
	if cause != "" {
		action += fmt.Sprintf("Cause: %s\r\n", cause)
	}
	action += "\r\n"

	return c.sendAction(action)
}

// GetChannels obtiene información de canales activos
func (c *Client) GetChannels() error {
	action := "Action: CoreShowChannels\r\n\r\n"
	return c.sendAction(action)
}
