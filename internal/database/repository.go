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
		       retry_time, amd_active, smart_cid_active, created_at, updated_at
		FROM apicall_proyectos
		WHERE id = ?
	`

	var p Proyecto
	err := r.conn.DB.QueryRow(query, id).Scan(
		&p.ID, &p.Nombre, &p.CallerID, &p.Audio, &p.DTMFEsperado,
		&p.NumeroDesborde, &p.TroncalSalida, &p.PrefijoSalida,
		&p.IPsAutorizadas, &p.MaxRetries, &p.RetryTime, &p.AMDActive, &p.SmartCIDActive,
		&p.CreatedAt, &p.UpdatedAt,
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
		       created_at, updated_at
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
			&p.CreatedAt, &p.UpdatedAt,
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

	query := `
		INSERT INTO apicall_proyectos (id, nombre, caller_id, audio, dtmf_esperado,
		                                numero_desborde, troncal_salida, prefijo_salida,
		                                ips_autorizadas, max_retries, retry_time, amd_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.conn.DB.Exec(query,
		p.ID, p.Nombre, p.CallerID, p.Audio, p.DTMFEsperado,
		p.NumeroDesborde, p.TroncalSalida, p.PrefijoSalida,
		p.IPsAutorizadas, p.MaxRetries, p.RetryTime, p.AMDActive,
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

// CreateCallLog registra una llamada
func (r *Repository) CreateCallLog(log *CallLog) (int64, error) {
	query := `
		INSERT INTO apicall_call_log (proyecto_id, telefono, status, interacciono, caller_id_used)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.conn.DB.Exec(query,
		log.ProyectoID, log.Telefono, log.Status, log.Interacciono, log.CallerIDUsed,
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
func (r *Repository) UpdateCallLog(id int64, dtmfMarcado *string, disposition *string, interacciono bool, status string, duracion int) error {
	// Optimization: Use Batcher instead of direct SQL
	update := LogUpdate{
		ID:           id,
		DTMFMarcado:  dtmfMarcado,
		Disposition:  disposition,
		Interacciono: interacciono,
		Status:       status,
		Duracion:     duracion,
	}
	r.batcher.Queue(update)
	return nil
}

// GetCallLogsByProyecto obtiene logs de llamadas por proyecto
func (r *Repository) GetCallLogsByProyecto(proyectoID int, limit int) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, dtmf_marcado, interacciono, status, disposition, duracion, uniqueid, caller_id_used, created_at
		FROM apicall_call_log
		WHERE proyecto_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.conn.DB.Query(query, proyectoID, limit)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs: %w", err)
	}
	defer rows.Close()

	var logs []CallLog
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetRecentCallLogs obtiene los logs más recientes sin filtrar por proyecto
func (r *Repository) GetRecentCallLogs(limit int) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, dtmf_marcado, interacciono, status, disposition, duracion, uniqueid, caller_id_used, created_at
		FROM apicall_call_log
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.conn.DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("error consultando logs: %w", err)
	}
	defer rows.Close()

	var logs []CallLog
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetCallLogsByProyectoWithDates obtiene logs de llamadas por proyecto con filtro de fechas
func (r *Repository) GetCallLogsByProyectoWithDates(proyectoID int, limit int, fromDate, toDate string) ([]CallLog, error) {
	query := `
		SELECT id, proyecto_id, telefono, dtmf_marcado, interacciono, status, disposition, duracion, uniqueid, caller_id_used, created_at
		FROM apicall_call_log
		WHERE proyecto_id = ?
	`

	args := []interface{}{proyectoID}

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

	var logs []CallLog
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CreatedAt,
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
		SELECT id, proyecto_id, telefono, dtmf_marcado, interacciono, status, disposition, duracion, uniqueid, caller_id_used, created_at
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

	var logs []CallLog
	for rows.Next() {
		var log CallLog
		err := rows.Scan(
			&log.ID, &log.ProyectoID, &log.Telefono, &log.DTMFMarcado,
			&log.Interacciono, &log.Status, &log.Disposition, &log.Duracion, &log.Uniqueid, &log.CallerIDUsed, &log.CreatedAt,
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
	query := `SELECT id, nombre, host, puerto, usuario, password, contexto, caller_id, activo FROM apicall_troncales`
	rows, err := r.conn.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error consultando troncales: %w", err)
	}
	defer rows.Close()

	var troncales []Troncal
	for rows.Next() {
		var t Troncal
		if err := rows.Scan(&t.ID, &t.Nombre, &t.Host, &t.Puerto, &t.Usuario, &t.Password, &t.Contexto, &t.CallerID, &t.Activo); err != nil {
			return nil, err
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
