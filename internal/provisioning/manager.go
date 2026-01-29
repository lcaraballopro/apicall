package provisioning

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"apicall/internal/config"
	"apicall/internal/sysadmin"
	
	_ "github.com/go-sql-driver/mysql"
)

// EnsureInfrastructure ensures DB and Asterisk are installed and running
func EnsureInfrastructure(cfg *config.Config) {
	// 1. Install/Ensure Asterisk
	installAsterisk()
	
	// 2. Configure Asterisk (Manager, Modules, Dialplan)
	ConfigureAsterisk(cfg)
	
	// Reload Asterisk to apply changes
	exec.Command("asterisk", "-rx", "core reload").Run()
	exec.Command("asterisk", "-rx", "module reload manager").Run()

	// 3. Ensure DB (Install MariaDB + Bootstrap + Migrations)
	EnsureDB(cfg)
}

// EnsureDB ensures the specific DB exists, installing MariaDB if necessary
func EnsureDB(cfg *config.Config) {
    // 1. Try to connect normally first
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Database.Username, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)

	db, err := sql.Open("mysql", dsn)
	if err == nil {
		if err := db.Ping(); err == nil {
			// Connection OK, verify schema (migrations) and return
			log.Println("[Provisioner] Conexión DB exitosa. Verificando esquema...")
			if err := RunMigrations(db, "/opt/apicall/migrations"); err != nil {
				log.Printf("[Provisioner] Warning: Error corriendo migraciones: %v", err)
			}
			db.Close()
			return
		}
	}
	db.Close()

	log.Println("[Provisioner] No se pudo conectar a la BD. Iniciando protocolo de aprovisionamiento...")

    // Only attempt auto-install if localhost
	if cfg.Database.Host != "127.0.0.1" && cfg.Database.Host != "localhost" {
		log.Println("[Provisioner] BD Remota no accesible. Omitiendo instalación local.")
		return
	}

	// 2. Install/Check MariaDB Service
	installMariaDB()

    // 3. Bootstrap DB (Create DB and User)
    bootstrapDB(cfg)
}

func installAsterisk() {
	_, err := exec.LookPath("asterisk")
	if err == nil {
		log.Println("[Provisioner] Asterisk detectado.")
		// Ensure service is running
		if err := exec.Command("systemctl", "is-active", "asterisk").Run(); err != nil {
             log.Println("[Provisioner] Servicio asterisk no activo. Intentando iniciar...")
             exec.Command("systemctl", "start", "asterisk").Run()
        }
		return
	}

	log.Println("[Provisioner] Asterisk no detectado. Iniciando instalación...")
	osType := sysadmin.DetectOS()
	var cmd *exec.Cmd

	switch osType {
	case sysadmin.Debian:
		exec.Command("apt-get", "update").Run()
		// Basic Asterisk + Sounds + MOH
		cmd = exec.Command("apt-get", "install", "-y", "asterisk", "asterisk-core-sounds-es", "asterisk-moh-opsound-wav")
	case sysadmin.RHEL:
		// Assumption: EPEL or repo is enabled. RHEL often needs 'asterisk' package.
		// Sounds might vary in naming convention.
		cmd = exec.Command("yum", "install", "-y", "asterisk", "asterisk-sounds-core-es-wav")
	case sysadmin.Suse:
		cmd = exec.Command("zypper", "--non-interactive", "install", "asterisk")
	default:
		log.Println("[Provisioner] OS no soportado para auto-instalación de Asterisk. Instale manualmente.")
		return
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[Provisioner] Error instalando Asterisk: %v", err)
		return
	}

	// Enable and Start
	exec.Command("systemctl", "enable", "--now", "asterisk").Run()
	log.Println("[Provisioner] Asterisk instalado y arrancado.")
	time.Sleep(5 * time.Second) // Allow startup
}

func installMariaDB() {
	// Check if mysql command exists
	_, err := exec.LookPath("mysql")
	if err == nil {
        // Check service status
        if err := exec.Command("systemctl", "is-active", "mariadb").Run(); err != nil {
             log.Println("[Provisioner] Servicio mariadb no activo. Intentando iniciar...")
             exec.Command("systemctl", "start", "mariadb").Run()
        }
		return
	}

	log.Println("[Provisioner] MariaDB no detectado. Instalando...")
	
	osType := sysadmin.DetectOS()
	var cmd *exec.Cmd

	switch osType {
	case sysadmin.Debian:
		exec.Command("apt-get", "update").Run()
		cmd = exec.Command("apt-get", "install", "-y", "mariadb-server")
	case sysadmin.RHEL:
		cmd = exec.Command("yum", "install", "-y", "mariadb-server")
	case sysadmin.Suse:
		// Try zypper
		cmd = exec.Command("zypper", "--non-interactive", "install", "mariadb")
	default:
		log.Println("[Provisioner] OS no soportado para auto-instalación. Instale MariaDB manualmente.")
		return
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[Provisioner] Error instalando MariaDB: %v", err)
		return
	}

	// Start service
	exec.Command("systemctl", "enable", "--now", "mariadb").Run()
	log.Println("[Provisioner] MariaDB instalado y arrancado.")
    // Wait for startup
    time.Sleep(5 * time.Second)
}

func bootstrapDB(cfg *config.Config) {
    // Try connecting as root (no pass assumption for fresh install)
    log.Println("[Provisioner] Intentando bootstraping de esquemas...")
    
    // Connect to mysql system db
    rootDSN := "root:@tcp(localhost:3306)/mysql"
    db, err := sql.Open("mysql", rootDSN)
    if err != nil {
         log.Printf("[Provisioner] Error preparando conexión root: %v", err)
         return
    }
    defer db.Close()

    // Check connection
    if err := db.Ping(); err != nil {
         // Maybe root has password? or configured differently
         log.Printf("[Provisioner] No se pudo conectar como root (sin pass): %v. Saltando bootstrap.", err)
         // Fallback: Check if we can connect as user if it was just a service down issue before
         return 
    }

    // Create Database
    _, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", cfg.Database.Database))
    if err != nil {
        log.Printf("[Provisioner] Error creando DB: %v", err)
    }

    // Create User and Grant
    // Note: This is basic. Ideally check if user exists.
    query := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%%' IDENTIFIED BY '%s' WITH GRANT OPTION", 
        cfg.Database.Database, cfg.Database.Username, cfg.Database.Password)
    
     _, err = db.Exec(query)
    if err != nil {
        log.Printf("[Provisioner] Error creando usuario: %v", err)
    }
    
    _, err = db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'localhost' IDENTIFIED BY '%s' WITH GRANT OPTION", 
        cfg.Database.Database, cfg.Database.Username, cfg.Database.Password))

    db.Exec("FLUSH PRIVILEGES")
    
    log.Println("[Provisioner] Bootstrap completado. BD y Usuario configurados.")
    
    // Run Migrations (now that DB exists)
    // Connect with the new user/db
    userDSN := fmt.Sprintf("%s:%s@tcp(localhost:3306)/%s", 
        cfg.Database.Username, cfg.Database.Password, cfg.Database.Database)
    
    userDB, err := sql.Open("mysql", userDSN)
    if err == nil {
         RunMigrations(userDB, "/opt/apicall/migrations")
         userDB.Close()
    }
}
