package database

import "time"

// Proyecto representa una campaña configurada
type Proyecto struct {
	ID             int       `db:"id" json:"id"`
	Nombre         string    `db:"nombre" json:"nombre"`
	CallerID       string    `db:"caller_id" json:"caller_id"`
	Audio          string    `db:"audio" json:"audio"`
	DTMFEsperado   string    `db:"dtmf_esperado" json:"dtmf_esperado"`
	NumeroDesborde string    `db:"numero_desborde" json:"numero_desborde"`
	TroncalSalida  string    `db:"troncal_salida" json:"troncal_salida"`
	PrefijoSalida  string    `db:"prefijo_salida" json:"prefijo_salida"`
	IPsAutorizadas string    `db:"ips_autorizadas" json:"ips_autorizadas"`
	MaxRetries     int       `db:"max_retries" json:"max_retries"`
	RetryTime      int       `db:"retry_time" json:"retry_time"`
	AMDActive      bool      `db:"amd_active" json:"amd_active"`
	SmartCIDActive bool      `db:"smart_cid_active" json:"smart_cid_active"`
	Timezone       string    `db:"timezone" json:"timezone"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// Troncal representa una troncal SIP
type Troncal struct {
	ID       int    `db:"id" json:"id"`
	Nombre   string `db:"nombre" json:"nombre"`
	Host     string `db:"host" json:"host"`
	Puerto   int    `db:"puerto" json:"puerto"`
	Usuario  string `db:"usuario" json:"usuario"`
	Password string `db:"password" json:"password"`
	Contexto string `db:"contexto" json:"contexto"`
	CallerID string `db:"caller_id" json:"caller_id"`
	Activo   bool   `db:"activo" json:"activo"`
}

// CallLog representa el registro de una llamada
type CallLog struct {
	ID           int64     `db:"id" json:"id"`
	ProyectoID   int       `db:"proyecto_id" json:"proyecto_id"`
	CampaignID   *int      `db:"campaign_id" json:"campaign_id,omitempty"` // Pointer to allow NULL in JSON/DB
	Telefono     string    `db:"telefono" json:"telefono"`
	DTMFMarcado  string    `db:"dtmf_marcado" json:"dtmf_marcado"`
	Interacciono bool      `db:"interacciono" json:"interacciono"`
	Status       string    `db:"status" json:"status"`
	Disposition  string    `db:"disposition" json:"disposition"`
	Duracion     int       `db:"duracion" json:"duracion"`
	Uniqueid     string    `db:"uniqueid" json:"uniqueid"`
	CallerIDUsed string    `db:"caller_id_used" json:"caller_id_used"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// Campaign representa una campaña masiva de llamadas
type Campaign struct {
	ID                 int       `db:"id" json:"id"`
	Nombre             string    `db:"nombre" json:"nombre"`
	ProyectoID         int       `db:"proyecto_id" json:"proyecto_id"`
	Estado             string    `db:"estado" json:"estado"` // draft, active, paused, completed, stopped
	TotalContactos     int       `db:"total_contactos" json:"total_contactos"`
	ContactosProcesados int     `db:"contactos_procesados" json:"contactos_procesados"`
	ContactosExitosos  int     `db:"contactos_exitosos" json:"contactos_exitosos"`
	ContactosFallidos  int     `db:"contactos_fallidos" json:"contactos_fallidos"`
	FechaInicio        *time.Time `db:"fecha_inicio" json:"fecha_inicio"`
	FechaFin           *time.Time `db:"fecha_fin" json:"fecha_fin"`
	CreatedAt          time.Time `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at" json:"updated_at"`
}

// CampaignContact representa un contacto (número) dentro de una campaña
type CampaignContact struct {
	ID              int64     `db:"id" json:"id"`
	CampaignID      int       `db:"campaign_id" json:"campaign_id"`
	Telefono        string    `db:"telefono" json:"telefono"`
	DatosAdicionales *string  `db:"datos_adicionales" json:"datos_adicionales"` // JSON string
	Estado          string    `db:"estado" json:"estado"` // pending, dialing, completed, failed, skipped
	Intentos        int       `db:"intentos" json:"intentos"`
	UltimoIntento   *time.Time `db:"ultimo_intento" json:"ultimo_intento"`
	Resultado       *string   `db:"resultado" json:"resultado"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
}

// CampaignSchedule representa un horario de campaña por día de la semana
type CampaignSchedule struct {
	ID         int       `db:"id" json:"id"`
	CampaignID int       `db:"campaign_id" json:"campaign_id"`
	DiaSemana  int       `db:"dia_semana" json:"dia_semana"` // 0=Domingo, 1=Lunes, ..., 6=Sábado
	HoraInicio string    `db:"hora_inicio" json:"hora_inicio"` // TIME format "HH:MM:SS"
	HoraFin    string    `db:"hora_fin" json:"hora_fin"` // TIME format "HH:MM:SS"
	Activo     bool      `db:"activo" json:"activo"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// BlacklistEntry representa un número bloqueado por proyecto
type BlacklistEntry struct {
	ID         int64     `db:"id" json:"id"`
	ProyectoID int       `db:"proyecto_id" json:"proyecto_id"`
	Telefono   string    `db:"telefono" json:"telefono"`
	Razon      *string   `db:"razon" json:"razon"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
