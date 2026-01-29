package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"apicall/internal/config"
)

// Connection maneja el pool de conexiones a la base de datos
type Connection struct {
	DB *sql.DB
}

// NewConnection crea una nueva conexión a la base de datos
func NewConnection(cfg config.DatabaseConfig) (*Connection, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("error abriendo conexión: %w", err)
	}

	// Configurar pool de conexiones
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	// Verificar conectividad
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error conectando a la base de datos: %w", err)
	}

	return &Connection{DB: db}, nil
}

// Close cierra la conexión a la base de datos
func (c *Connection) Close() error {
	return c.DB.Close()
}
