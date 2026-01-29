package provisioning

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations executes SQL migration files in order
func RunMigrations(db *sql.DB, migrationsPath string) error {
	log.Printf("[Provisioner] Buscando migraciones en %s", migrationsPath)

	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("error leyendo directorio de migraciones: %w", err)
	}

	var sqlFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}

	sort.Strings(sqlFiles)

	for _, filename := range sqlFiles {
		log.Printf("[Provisioner] Ejecutando migración: %s", filename)
		content, err := os.ReadFile(filepath.Join(migrationsPath, filename))
		if err != nil {
			return fmt.Errorf("error leyendo archivo %s: %w", filename, err)
		}

		queries := strings.Split(string(content), ";")
		for _, q := range queries {
			q = strings.TrimSpace(q)
			if q == "" {
				continue
			}
			if _, err := db.Exec(q); err != nil {
				// Ignore "already exists" errors for idempotency if simple
				// But ideally better migration logic checks existence.
				// For now, let's assume valid SQL or ignore specific errors casually:
				if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "Duplicate column") {
					continue 
				}
				return fmt.Errorf("error ejecutando query en %s: %w", filename, err)
			}
		}
	}
	return nil
}
