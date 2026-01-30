package database

import (
	"database/sql"
	"fmt"
)

// Repository maneja las operaciones de base de datos
type Repository struct {
	conn    *Connection
	batcher *LogBatcher
}

// NewRepository crea un nuevo repositorio
func NewRepository(conn *Connection) *Repository {
	repo := &Repository{
		conn:    conn,
		batcher: NewLogBatcher(conn.DB),
	}
	repo.batcher.Start()
	return repo
}

// Close cierra recursos del repositorio
func (r *Repository) Close() {
	if r.batcher != nil {
		r.batcher.Stop()
	}
}

// GetDB returns the underlying sql.DB
func (r *Repository) GetDB() *sql.DB {
	return r.conn.DB
}

// GetProyecto obtiene un proyecto por ID
func (r *Repository) GetProyecto(id int) (*Proyecto, error) {
	query := `
		SELECT id, nombre, caller_id, audio, dtmf_esperado, numero_desborde,
		       troncal_salida, prefijo_salida, ips_autorizadas, max_retries,
		       retry_time, amd_active, smart_cid_active, COALESCE(timezone, 'America/Bogota'), created_at, updated_at
		FROM apicall_proyectos
		WHERE id = ?
	`

	var p Proyecto
	err := r.conn.DB.QueryRow(query, id).Scan(
		&p.ID, &p.Nombre, &p.CallerID, &p.Audio, &p.DTMFEsperado,
		&p.NumeroDesborde, &p.TroncalSalida, &p.PrefijoSalida,
		&p.IPsAutorizadas, &p.MaxRetries, &p.RetryTime, &p.AMDActive, &p.SmartCIDActive,
		&p.Timezone, &p.CreatedAt, &p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("proyecto %d no encontrado", id)
	}
	if err != nil {
		return nil, fmt.Errorf("error consultando proyecto: %w", err)
	}

	return &p, nil
}

// ListProyectos lista todos los proyectos
func (r *Repository) ListProyectos() ([]Proyecto, error) {
	query := `
		SELECT id, nombre, caller_id, audio, dtmf_esperado, numero_desborde,
		       troncal_salida, prefijo_salida, ips_autorizadas, max_retries, retry_time, amd_active,
		       smart_cid_active, COALESCE(timezone, 'America/Bogota'), created_at, updated_at
		FROM apicall_proyectos
		ORDER BY id
	`

	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error listando proyectos: %w", err)
	}
	defer rows.Close()

	var proyectos []Proyecto
	for rows.Next() {
		var p Proyecto
		err := rows.Scan(
			&p.ID, &p.Nombre, &p.CallerID, &p.Audio, &p.DTMFEsperado,
			&p.NumeroDesborde, &p.TroncalSalida, &p.PrefijoSalida,
			&p.IPsAutorizadas, &p.MaxRetries, &p.RetryTime, &p.AMDActive,
			&p.SmartCIDActive, &p.Timezone, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando proyecto: %w", err)
		}
		proyectos = append(proyectos, p)
	}

	return proyectos, nil
}

// CreateProyecto crea un nuevo proyecto
func (r *Repository) CreateProyecto(p *Proyecto) error {
	// Valores por defecto si no se especifican
	if p.MaxRetries == 0 {
		p.MaxRetries = 2
	}
	if p.RetryTime == 0 {
		p.RetryTime = 60
	}

	// Default timezone if not specified
	if p.Timezone == "" {
		p.Timezone = "America/Bogota"
	}

	query := `
		INSERT INTO apicall_proyectos (id, nombre, caller_id, audio, dtmf_esperado,
		                                numero_desborde, troncal_salida, prefijo_salida,
		                                ips_autorizadas, max_retries, retry_time, amd_active, timezone)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.conn.DB.Exec(query,
		p.ID, p.Nombre, p.CallerID, p.Audio, p.DTMFEsperado,
		p.NumeroDesborde, p.TroncalSalida, p.PrefijoSalida,
		p.IPsAutorizadas, p.MaxRetries, p.RetryTime, p.AMDActive, p.Timezone,
	)

	if err != nil {
		return fmt.Errorf("error creando proyecto: %w", err)
	}

	return nil
}

// DeleteProyecto elimina un proyecto
func (r *Repository) DeleteProyecto(id int) error {
	query := `DELETE FROM apicall_proyectos WHERE id = ?`

	result, err := r.conn.DB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error eliminando proyecto: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("proyecto %d no encontrado", id)
	}

	return nil
}

// UpdateProyecto actualiza un proyecto existente
func (r *Repository) UpdateProyecto(p *Proyecto) error {
	query := `
		UPDATE apicall_proyectos 
		SET nombre = ?, caller_id = ?, audio = ?, dtmf_esperado = ?,
		    numero_desborde = ?, troncal_salida = ?, prefijo_salida = ?,
		    ips_autorizadas = ?, max_retries = ?, retry_time = ?, 
		    amd_active = ?, smart_cid_active = ?, timezone = ?, updated_at = NOW()
		WHERE id = ?
	`

	result, err := r.conn.DB.Exec(query,
		p.Nombre, p.CallerID, p.Audio, p.DTMFEsperado,
		p.NumeroDesborde, p.TroncalSalida, p.PrefijoSalida,
		p.IPsAutorizadas, p.MaxRetries, p.RetryTime, p.AMDActive, p.SmartCIDActive, p.Timezone,
		p.ID,
	)

	if err != nil {
		return fmt.Errorf("error actualizando proyecto: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("proyecto %d no encontrado", p.ID)
	}

	return nil
}


// CreateCallLog registra una llamada
func (r *Repository) CreateCallLog(log *CallLog) (int64, error) {
	query := `
		INSERT INTO apicall_call_log (proyecto_id, telefono, status, interacciono, caller_id_used, campaign_id, uniqueid)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.conn.DB.Exec(query,
		log.ProyectoID, log.Telefono, log.Status, log.Interacciono, log.CallerIDUsed, log.CampaignID, log.Uniqueid,
	)

	if err != nil {
		return 0, fmt.Errorf("error creando log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error obteniendo ID: %w", err)
	}

	return id, nil
}

// UpdateCallLog actualiza un registro de llamada
func (r *Repository) UpdateCallLog(id int64, dtmfMarcado *string, disposition *string, uniqueid *string, interacciono bool, status string, duracion int) error {
	// Optimization: Use Batcher instead of direct SQL
	update := LogUpdate{
		ID:           id,
		DTMFMarcado:  dtmfMarcado,
		Disposition:  disposition,
		Uniqueid:     uniqueid,
		Interacciono: interacciono,
		Status:       status,
		Duracion:     duracion,
	}
	r.batcher.Queue(update)
	return nil
}

// GetCallLogsByProyecto obtiene logs de llamadas por proyecto
func (r *Repository) GetCallLogsByProyecto(proyectoID int, campaignID *int, limit int) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, COALESCE(dtmf_marcado, ''), interacciono, status, COALESCE(disposition, ''), duracion, COALESCE(uniqueid, ''), COALESCE(caller_id_used, ''), campaign_id, created_at
		FROM apicall_call_log
		WHERE proyecto_id = ?
	`
	args := []interface{}{proyectoID}

	if campaignID != nil {
		query += " AND campaign_id = ?"
		args = append(args, *campaignID)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.conn.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs: %w", err)
	}
	defer rows.Close()

	logs := make([]CallLog, 0)
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CampaignID, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// UpdateDialingCallByUniqueid updates a call that's still in DIALING status
// This is called by the AMI event handler when a call ends without reaching FastAGI
func (r *Repository) UpdateDialingCallByUniqueid(uniqueid string, status string, disposition string) (bool, error) {
	// Only update if the call is still in DIALING status
	// This prevents overwriting updates from FastAGI
	query := `
		UPDATE apicall_call_log 
		SET status = ?, disposition = ?
		WHERE status = 'DIALING' 
		  AND created_at > NOW() - INTERVAL 10 MINUTE
		  AND (uniqueid = ? OR uniqueid LIKE ?)
		LIMIT 1
	`
	
	result, err := r.conn.DB.Exec(query, status, disposition, uniqueid, "%"+uniqueid+"%")
	if err != nil {
		return false, err
	}
	
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

// GetRecentCallLogs obtiene los logs más recientes sin filtrar por proyecto
func (r *Repository) GetRecentCallLogs(limit int) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, COALESCE(dtmf_marcado, ''), interacciono, status, COALESCE(disposition, ''), duracion, COALESCE(uniqueid, ''), COALESCE(caller_id_used, ''), campaign_id, created_at
		FROM apicall_call_log
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.conn.DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs: %w", err)
	}
	defer rows.Close()

	logs := make([]CallLog, 0)
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CampaignID, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetCallLogsByProyectoWithDates obtiene logs de llamadas por proyecto con filtro de fechas
func (r *Repository) GetCallLogsByProyectoWithDates(proyectoID int, campaignID *int, limit int, fromDate, toDate string) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, COALESCE(dtmf_marcado, ''), interacciono, status, COALESCE(disposition, ''), duracion, COALESCE(uniqueid, ''), COALESCE(caller_id_used, ''), campaign_id, created_at
		FROM apicall_call_log
		WHERE proyecto_id = ?
	`

	args := []interface{}{proyectoID}

	if campaignID != nil {
		query += " AND campaign_id = ?"
		args = append(args, *campaignID)
	}

	if fromDate != "" {
		query += " AND DATE(created_at) >= ?"
		args = append(args, fromDate)
	}

	if toDate != "" {
		query += " AND DATE(created_at) <= ?"
		args = append(args, toDate)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.conn.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs: %w", err)
	}
	defer rows.Close()

	logs := make([]CallLog, 0)
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CampaignID, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetRecentCallLogsWithDates obtiene los logs más recientes con filtro de fechas
func (r *Repository) GetRecentCallLogsWithDates(limit int, fromDate, toDate string) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, COALESCE(dtmf_marcado, ''), interacciono, status, COALESCE(disposition, ''), duracion, COALESCE(uniqueid, ''), COALESCE(caller_id_used, ''), campaign_id, created_at
		FROM apicall_call_log
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND DATE(created_at) >= ?"
		args = append(args, fromDate)
	}

	if toDate != "" {
		query += " AND DATE(created_at) <= ?"
		args = append(args, toDate)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.conn.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs: %w", err)
	}
	defer rows.Close()

	logs := make([]CallLog, 0)
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CampaignID, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// CreateTroncal crea una nueva troncal
func (r *Repository) CreateTroncal(troncal *Troncal) error {
	query := `INSERT INTO apicall_troncales (nombre, host, puerto, usuario, password, contexto, caller_id, activo) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := r.conn.DB.Exec(query, troncal.Nombre, troncal.Host, troncal.Puerto, troncal.Usuario, troncal.Password, troncal.Contexto, troncal.CallerID, troncal.Activo)
	if err != nil {
		return fmt.Errorf("error insertando troncal: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	troncal.ID = int(id)
	return nil
}

// ListTroncales devuelve todas las troncales
func (r *Repository) ListTroncales() ([]Troncal, error) {
	query := `SELECT id, nombre, host, puerto, COALESCE(usuario, ''), COALESCE(password, ''), contexto, COALESCE(caller_id, ''), activo FROM apicall_troncales`
	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error consultando troncales: %w", err)
	}
	defer rows.Close()

	var troncales []Troncal
	for rows.Next() {
		var t Troncal
		if err := rows.Scan(&t.ID, &t.Nombre, &t.Host, &t.Puerto, &t.Usuario, &t.Password, &t.Contexto, &t.CallerID, &t.Activo); err != nil {
			return nil, fmt.Errorf("error escaneando troncal: %w", err)
		}
		troncales = append(troncales, t)
	}
	return troncales, nil
}

// DeleteTroncal elimina una troncal
func (r *Repository) DeleteTroncal(id int) error {
	_, err := r.conn.DB.Exec("DELETE FROM apicall_troncales WHERE id = ?", id)
	return err
}

// GetConfig obtiene un valor de configuración por clave
func (r *Repository) GetConfig(key string) (string, error) {
	query := `SELECT config_value FROM apicall_config WHERE config_key = ?`
	var value string
	err := r.conn.DB.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if not found, not error
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetConfig establece o actualiza un valor de configuración
func (r *Repository) SetConfig(key, value, description string) error {
	query := `
		INSERT INTO apicall_config (config_key, config_value, description)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE 
			config_value = VALUES(config_value),
			description = COALESCE(VALUES(description), description)
	`
	_, err := r.conn.DB.Exec(query, key, value, description)
	return err
}

// Config represents a system configuration entry
type Config struct {
	ID          int    `db:"id" json:"id"`
	Key         string `db:"config_key" json:"key"`
	Value       string `db:"config_value" json:"value"`
	Description string `db:"description" json:"description"`
}

// ListConfigs returns all system configurations
func (r *Repository) ListConfigs() ([]Config, error) {
	query := `SELECT id, config_key, config_value, COALESCE(description, '') as description FROM apicall_config ORDER BY config_key`
	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []Config
	for rows.Next() {
		var c Config
		if err := rows.Scan(&c.ID, &c.Key, &c.Value, &c.Description); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, nil
}

// AssignTroncalToProyecto vincula una troncal a un proyecto
func (r *Repository) AssignTroncalToProyecto(proyectoID, troncalID int) error {
	query := `INSERT IGNORE INTO apicall_proyecto_troncal (proyecto_id, troncal_id) VALUES (?, ?)`
	_, err := r.conn.DB.Exec(query, proyectoID, troncalID)
	return err
}

// RemoveTroncalFromProyecto desvincula una troncal
func (r *Repository) RemoveTroncalFromProyecto(proyectoID, troncalID int) error {
	query := `DELETE FROM apicall_proyecto_troncal WHERE proyecto_id = ? AND troncal_id = ?`
	_, err := r.conn.DB.Exec(query, proyectoID, troncalID)
	return err
}

// GetTroncalesNamesByProyecto retorna los nombres de las troncales asignadas a un proyecto
func (r *Repository) GetTroncalesNamesByProyecto(proyectoID int) ([]string, error) {
	query := `
        SELECT t.nombre 
        FROM apicall_troncales t
        JOIN apicall_proyecto_troncal pt ON t.id = pt.troncal_id
        WHERE pt.proyecto_id = ? AND t.activo = TRUE
    `
	rows, err := r.conn.DB.Query(query, proyectoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// --- USER MANAGEMENT ---

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	FullName     string `json:"full_name"`
	Active       bool   `json:"active"`
}

func (r *Repository) GetUserByUsername(username string) (*User, error) {
	query := `SELECT id, username, password_hash, role, full_name, active FROM users WHERE username = ?`
	row := r.conn.DB.QueryRow(query, username)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.FullName, &u.Active)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CreateUser(u *User) error {
	query := `INSERT INTO users (username, password_hash, role, full_name) VALUES (?, ?, ?, ?)`
	_, err := r.conn.DB.Exec(query, u.Username, u.PasswordHash, u.Role, u.FullName)
	return err
}

func (r *Repository) ListUsers() ([]User, error) {
	query := `SELECT id, username, role, full_name, active, created_at FROM users`
	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var createdAt string // Placeholder
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.FullName, &u.Active, &createdAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *Repository) DeleteUser(id int) error {
	_, err := r.conn.DB.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// --- BLACKLIST MANAGEMENT ---

// IsBlacklisted verifica si un número está bloqueado para un proyecto
func (r *Repository) IsBlacklisted(proyectoID int, telefono string) (bool, error) {
	query := `SELECT COUNT(*) FROM apicall_blacklist WHERE proyecto_id = ? AND telefono = ?`
	var count int
	err := r.conn.DB.QueryRow(query, proyectoID, telefono).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AddToBlacklist agrega un número a la lista negra
func (r *Repository) AddToBlacklist(entry *BlacklistEntry) error {
	query := `INSERT INTO apicall_blacklist (proyecto_id, telefono, razon) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE razon = VALUES(razon)`
	_, err := r.conn.DB.Exec(query, entry.ProyectoID, entry.Telefono, entry.Razon)
	return err
}

// AddToBlacklistBulk agrega múltiples números a la lista negra
func (r *Repository) AddToBlacklistBulk(proyectoID int, telefonos []string) (int, error) {
	if len(telefonos) == 0 {
		return 0, nil
	}

	tx, err := r.conn.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO apicall_blacklist (proyecto_id, telefono) VALUES (?, ?) ON DUPLICATE KEY UPDATE telefono = telefono`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, tel := range telefonos {
		if tel == "" {
			continue
		}
		_, err := stmt.Exec(proyectoID, tel)
		if err != nil {
			continue // Skip duplicates or errors
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// ListBlacklist lista los números bloqueados para un proyecto
func (r *Repository) ListBlacklist(proyectoID int, limit int) ([]BlacklistEntry, error) {
	query := `SELECT id, proyecto_id, telefono, razon, created_at FROM apicall_blacklist WHERE proyecto_id = ? ORDER BY created_at DESC LIMIT ?`
	rows, err := r.conn.DB.Query(query, proyectoID, limit)
	if err != nil {
		return nil, fmt.Errorf("error consultando blacklist: %w", err)
	}
	defer rows.Close()

	var entries []BlacklistEntry
	for rows.Next() {
		var e BlacklistEntry
		if err := rows.Scan(&e.ID, &e.ProyectoID, &e.Telefono, &e.Razon, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("error escaneando blacklist: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// DeleteFromBlacklist elimina un número de la lista negra
func (r *Repository) DeleteFromBlacklist(id int64) error {
	_, err := r.conn.DB.Exec("DELETE FROM apicall_blacklist WHERE id = ?", id)
	return err
}

// ClearBlacklist elimina todos los números bloqueados de un proyecto
func (r *Repository) ClearBlacklist(proyectoID int) error {
	_, err := r.conn.DB.Exec("DELETE FROM apicall_blacklist WHERE proyecto_id = ?", proyectoID)
	return err
}

// CountBlacklist cuenta los números bloqueados de un proyecto
func (r *Repository) CountBlacklist(proyectoID int) (int, error) {
	query := `SELECT COUNT(*) FROM apicall_blacklist WHERE proyecto_id = ?`
	var count int
	err := r.conn.DB.QueryRow(query, proyectoID).Scan(&count)
	return count, err
}

// --- CAMPAIGN MANAGEMENT ---

// CreateCampaign crea una nueva campaña masiva
func (r *Repository) CreateCampaign(c *Campaign) error {
	query := `
		INSERT INTO apicall_campaigns (nombre, proyecto_id, estado, total_contactos)
		VALUES (?, ?, ?, ?)
	`
	res, err := r.conn.DB.Exec(query, c.Nombre, c.ProyectoID, c.Estado, c.TotalContactos)
	if err != nil {
		return fmt.Errorf("error creando campaña: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = int(id)
	return nil
}

// GetCampaign obtiene una campaña por ID
func (r *Repository) GetCampaign(id int) (*Campaign, error) {
	query := `
		SELECT id, nombre, proyecto_id, estado, total_contactos, contactos_procesados,
		       contactos_exitosos, contactos_fallidos, fecha_inicio, fecha_fin,
		       created_at, updated_at
		FROM apicall_campaigns
		WHERE id = ?
	`
	var c Campaign
	err := r.conn.DB.QueryRow(query, id).Scan(
		&c.ID, &c.Nombre, &c.ProyectoID, &c.Estado, &c.TotalContactos,
		&c.ContactosProcesados, &c.ContactosExitosos, &c.ContactosFallidos,
		&c.FechaInicio, &c.FechaFin, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("campaña %d no encontrada", id)
	}
	if err != nil {
		return nil, fmt.Errorf("error consultando campaña: %w", err)
	}
	return &c, nil
}

// ListCampaigns lista todas las campañas
func (r *Repository) ListCampaigns() ([]Campaign, error) {
	query := `
		SELECT id, nombre, proyecto_id, estado, total_contactos, contactos_procesados,
		       contactos_exitosos, contactos_fallidos, fecha_inicio, fecha_fin,
		       created_at, updated_at
		FROM apicall_campaigns
		ORDER BY created_at DESC
	`
	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error listando campañas: %w", err)
	}
	defer rows.Close()

	campaigns := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		err := rows.Scan(
			&c.ID, &c.Nombre, &c.ProyectoID, &c.Estado, &c.TotalContactos,
			&c.ContactosProcesados, &c.ContactosExitosos, &c.ContactosFallidos,
			&c.FechaInicio, &c.FechaFin, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando campaña: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

// ListCampaignsByProyecto lista campañas de un proyecto específico
func (r *Repository) ListCampaignsByProyecto(proyectoID int) ([]Campaign, error) {
	query := `
		SELECT id, nombre, proyecto_id, estado, total_contactos, contactos_procesados,
		       contactos_exitosos, contactos_fallidos, fecha_inicio, fecha_fin,
		       created_at, updated_at
		FROM apicall_campaigns
		WHERE proyecto_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.conn.DB.Query(query, proyectoID)
	if err != nil {
		return nil, fmt.Errorf("error listando campañas: %w", err)
	}
	defer rows.Close()

	campaigns := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		err := rows.Scan(
			&c.ID, &c.Nombre, &c.ProyectoID, &c.Estado, &c.TotalContactos,
			&c.ContactosProcesados, &c.ContactosExitosos, &c.ContactosFallidos,
			&c.FechaInicio, &c.FechaFin, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando campaña: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

// UpdateCampaign actualiza una campaña
func (r *Repository) UpdateCampaign(c *Campaign) error {
	query := `
		UPDATE apicall_campaigns 
		SET nombre = ?, estado = ?, updated_at = NOW()
		WHERE id = ?
	`
	result, err := r.conn.DB.Exec(query, c.Nombre, c.Estado, c.ID)
	if err != nil {
		return fmt.Errorf("error actualizando campaña: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("campaña %d no encontrada", c.ID)
	}
	return nil
}

// UpdateCampaignStatus actualiza solo el estado de una campaña
func (r *Repository) UpdateCampaignStatus(id int, estado string) error {
	query := `UPDATE apicall_campaigns SET estado = ?, updated_at = NOW() WHERE id = ?`
	if estado == "active" {
		query = `UPDATE apicall_campaigns SET estado = ?, fecha_inicio = COALESCE(fecha_inicio, NOW()), updated_at = NOW() WHERE id = ?`
	} else if estado == "completed" || estado == "stopped" {
		query = `UPDATE apicall_campaigns SET estado = ?, fecha_fin = NOW(), updated_at = NOW() WHERE id = ?`
	}
	_, err := r.conn.DB.Exec(query, estado, id)
	return err
}

// UpdateCampaignStats actualiza las estadísticas de contactos procesados
func (r *Repository) UpdateCampaignStats(id int, processed, success, failed int) error {
	query := `
		UPDATE apicall_campaigns 
		SET contactos_procesados = ?, contactos_exitosos = ?, contactos_fallidos = ?, updated_at = NOW()
		WHERE id = ?
	`
	_, err := r.conn.DB.Exec(query, processed, success, failed, id)
	return err
}

// DeleteCampaign elimina una campaña y sus contactos/schedules (cascade)
func (r *Repository) DeleteCampaign(id int) error {
	query := `DELETE FROM apicall_campaigns WHERE id = ?`
	result, err := r.conn.DB.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error eliminando campaña: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("campaña %d no encontrada", id)
	}
	return nil
}

// GetActiveCampaigns obtiene todas las campañas activas (para sweeper)
func (r *Repository) GetActiveCampaigns() ([]Campaign, error) {
	query := `
		SELECT id, nombre, proyecto_id, estado, total_contactos, contactos_procesados,
		       contactos_exitosos, contactos_fallidos, fecha_inicio, fecha_fin,
		       created_at, updated_at
		FROM apicall_campaigns
		WHERE estado = 'active'
	`
	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error listando campañas activas: %w", err)
	}
	defer rows.Close()

	campaigns := make([]Campaign, 0)
	for rows.Next() {
		var c Campaign
		err := rows.Scan(
			&c.ID, &c.Nombre, &c.ProyectoID, &c.Estado, &c.TotalContactos,
			&c.ContactosProcesados, &c.ContactosExitosos, &c.ContactosFallidos,
			&c.FechaInicio, &c.FechaFin, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando campaña: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

// --- CAMPAIGN CONTACTS ---

// CreateCampaignContactsBulk inserta contactos en batches de 1000
func (r *Repository) CreateCampaignContactsBulk(campaignID int, telefonos []string) (int, error) {
	if len(telefonos) == 0 {
		return 0, nil
	}

	const batchSize = 1000
	inserted := 0

	tx, err := r.conn.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO apicall_campaign_contacts (campaign_id, telefono, estado) VALUES (?, ?, 'pending')`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for i, tel := range telefonos {
		if tel == "" {
			continue
		}
		_, err := stmt.Exec(campaignID, tel)
		if err != nil {
			continue // Skip errors (duplicates, etc)
		}
		inserted++

		// Commit in batches to avoid long transactions
		if (i+1)%batchSize == 0 {
			if err := tx.Commit(); err != nil {
				return inserted, err
			}
			tx, err = r.conn.DB.Begin()
			if err != nil {
				return inserted, err
			}
			stmt, err = tx.Prepare(`INSERT INTO apicall_campaign_contacts (campaign_id, telefono, estado) VALUES (?, ?, 'pending')`)
			if err != nil {
				return inserted, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return inserted, err
	}

	// Update campaign total
	r.conn.DB.Exec(`UPDATE apicall_campaigns SET total_contactos = ? WHERE id = ?`, inserted, campaignID)

	return inserted, nil
}

// GetPendingContacts obtiene contactos pendientes para procesar
func (r *Repository) GetPendingContacts(campaignID int, limit int) ([]CampaignContact, error) {
	query := `
		SELECT id, campaign_id, telefono, datos_adicionales, estado, intentos, ultimo_intento, resultado, created_at
		FROM apicall_campaign_contacts
		WHERE campaign_id = ? AND estado = 'pending'
		ORDER BY id
		LIMIT ?
	`
	rows, err := r.conn.DB.Query(query, campaignID, limit)
	if err != nil {
		return nil, fmt.Errorf("error consultando contactos: %w", err)
	}
	defer rows.Close()

	contacts := make([]CampaignContact, 0)
	for rows.Next() {
		var c CampaignContact
		err := rows.Scan(
			&c.ID, &c.CampaignID, &c.Telefono, &c.DatosAdicionales,
			&c.Estado, &c.Intentos, &c.UltimoIntento, &c.Resultado, &c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando contacto: %w", err)
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

// UpdateContactStatus actualiza el estado de un contacto
func (r *Repository) UpdateContactStatus(id int64, estado string, resultado *string) error {
	query := `UPDATE apicall_campaign_contacts SET estado = ?, resultado = ?, ultimo_intento = NOW(), intentos = intentos + 1 WHERE id = ?`
	_, err := r.conn.DB.Exec(query, estado, resultado, id)
	return err
}

// MarkContactDialing marca un contacto como "dialing"
func (r *Repository) MarkContactDialing(id int64) error {
	query := `UPDATE apicall_campaign_contacts SET estado = 'dialing', ultimo_intento = NOW() WHERE id = ?`
	_, err := r.conn.DB.Exec(query, id)
	return err
}

// CountContactsByStatus cuenta contactos por estado
func (r *Repository) CountContactsByStatus(campaignID int) (map[string]int, error) {
	query := `
		SELECT estado, COUNT(*) as cnt
		FROM apicall_campaign_contacts
		WHERE campaign_id = ?
		GROUP BY estado
	`
	rows, err := r.conn.DB.Query(query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var estado string
		var cnt int
		if err := rows.Scan(&estado, &cnt); err != nil {
			return nil, err
		}
		counts[estado] = cnt
	}
	return counts, nil
}

// --- CAMPAIGN SCHEDULES ---

// CreateCampaignSchedule crea un horario de campaña
func (r *Repository) CreateCampaignSchedule(s *CampaignSchedule) error {
	query := `
		INSERT INTO apicall_campaign_schedules (campaign_id, dia_semana, hora_inicio, hora_fin, activo)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE hora_inicio = VALUES(hora_inicio), hora_fin = VALUES(hora_fin), activo = VALUES(activo)
	`
	_, err := r.conn.DB.Exec(query, s.CampaignID, s.DiaSemana, s.HoraInicio, s.HoraFin, s.Activo)
	return err
}

// GetCampaignSchedules obtiene los horarios de una campaña
func (r *Repository) GetCampaignSchedules(campaignID int) ([]CampaignSchedule, error) {
	query := `
		SELECT id, campaign_id, dia_semana, hora_inicio, hora_fin, activo, created_at
		FROM apicall_campaign_schedules
		WHERE campaign_id = ?
		ORDER BY dia_semana
	`
	rows, err := r.conn.DB.Query(query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := make([]CampaignSchedule, 0)
	for rows.Next() {
		var s CampaignSchedule
		if err := rows.Scan(&s.ID, &s.CampaignID, &s.DiaSemana, &s.HoraInicio, &s.HoraFin, &s.Activo, &s.CreatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// UpdateCampaignSchedules reemplaza todos los schedules de una campaña
func (r *Repository) UpdateCampaignSchedules(campaignID int, schedules []CampaignSchedule) error {
	tx, err := r.conn.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing schedules
	_, err = tx.Exec(`DELETE FROM apicall_campaign_schedules WHERE campaign_id = ?`, campaignID)
	if err != nil {
		return err
	}

	// Insert new schedules
	stmt, err := tx.Prepare(`
		INSERT INTO apicall_campaign_schedules (campaign_id, dia_semana, hora_inicio, hora_fin, activo)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range schedules {
		_, err = stmt.Exec(campaignID, s.DiaSemana, s.HoraInicio, s.HoraFin, s.Activo)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// IsWithinSchedule verifica si la hora actual está dentro del horario de la campaña
func (r *Repository) IsWithinSchedule(campaignID int) (bool, error) {
	// MySQL: DAYOFWEEK returns 1=Sunday, 2=Monday, etc. We need to map to our 0=Sunday format
	query := `
		SELECT COUNT(*) FROM apicall_campaign_schedules
		WHERE campaign_id = ?
		  AND activo = TRUE
		  AND dia_semana = (DAYOFWEEK(NOW()) - 1)
		  AND CURTIME() BETWEEN hora_inicio AND hora_fin
	`
	var count int
	err := r.conn.DB.QueryRow(query, campaignID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// --- CAMPAIGN RECYCLING ---

// DispositionCount representa el conteo de contactos por resultado
type DispositionCount struct {
	Resultado string `json:"resultado"`
	Count     int    `json:"count"`
}

// CountContactsByResultado cuenta contactos agrupados por resultado/disposición
func (r *Repository) CountContactsByResultado(campaignID int) ([]DispositionCount, error) {
	query := `
		SELECT COALESCE(resultado, 'PENDING') as resultado, COUNT(*) as cnt
		FROM apicall_campaign_contacts
		WHERE campaign_id = ?
		GROUP BY resultado
		ORDER BY cnt DESC
	`
	rows, err := r.conn.DB.Query(query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("error contando contactos por resultado: %w", err)
	}
	defer rows.Close()

	var counts []DispositionCount
	for rows.Next() {
		var dc DispositionCount
		if err := rows.Scan(&dc.Resultado, &dc.Count); err != nil {
			return nil, fmt.Errorf("error escaneando conteo: %w", err)
		}
		counts = append(counts, dc)
	}
	return counts, nil
}

// RecycleCampaignContacts copia contactos de una campaña origen a una nueva, filtrados por resultados
func (r *Repository) RecycleCampaignContacts(sourceCampaignID, targetCampaignID int, resultados []string) (int, error) {
	if len(resultados) == 0 {
		return 0, nil
	}

	// Construir placeholders para IN clause
	placeholders := ""
	args := make([]interface{}, 0, len(resultados)+2)
	args = append(args, targetCampaignID, sourceCampaignID)
	for i, res := range resultados {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, res)
	}

	query := fmt.Sprintf(`
		INSERT INTO apicall_campaign_contacts (campaign_id, telefono, datos_adicionales, estado)
		SELECT ?, telefono, datos_adicionales, 'pending'
		FROM apicall_campaign_contacts
		WHERE campaign_id = ? AND COALESCE(resultado, 'PENDING') IN (%s)
	`, placeholders)

	result, err := r.conn.DB.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("error reciclando contactos: %w", err)
	}

	inserted, _ := result.RowsAffected()

	// Actualizar total de contactos en la nueva campaña
	r.conn.DB.Exec(`UPDATE apicall_campaigns SET total_contactos = ? WHERE id = ?`, inserted, targetCampaignID)

	return int(inserted), nil
}
