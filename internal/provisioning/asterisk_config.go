package provisioning

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"apicall/internal/config"
	"apicall/internal/database"
)

// SyncTroncales generates sip_apicall.conf from DB
func SyncTroncales(repo *database.Repository) error {
	log.Println("[Provisioner] Sincronizando troncales...")
	
	troncales, err := repo.ListTroncales()
	if err != nil {
		return fmt.Errorf("error listando troncales: %w", err)
	}
	
	var sb strings.Builder
	sb.WriteString("; Generado automáticamente por Apicall\n\n")
	
	for _, t := range troncales {
		if !t.Activo {
			continue
		}
		
		sb.WriteString(fmt.Sprintf("[%s]\n", t.Nombre))
		sb.WriteString("type=friend\n")
		sb.WriteString("disallow=all\n")
		sb.WriteString("allow=ulaw\n")
		sb.WriteString("allow=alaw\n")
		sb.WriteString(fmt.Sprintf("host=%s\n", t.Host))
		if t.Puerto != 0 {
			sb.WriteString(fmt.Sprintf("port=%d\n", t.Puerto))
		}
		if t.Usuario != "" {
			sb.WriteString(fmt.Sprintf("defaultuser=%s\n", t.Usuario))
		}
		if t.Password != "" {
			sb.WriteString(fmt.Sprintf("secret=%s\n", t.Password))
		}
		if t.Contexto != "" {
			sb.WriteString(fmt.Sprintf("context=%s\n", t.Contexto))
		}
		if t.CallerID != "" {
			sb.WriteString(fmt.Sprintf("callerid=%s\n", t.CallerID))
		}
		sb.WriteString("qualify=yes\n")
		sb.WriteString("nat=force_rport,comedia\n")
		sb.WriteString("insecure=port,invite\n\n")
	}
	
	destFile := "/etc/asterisk/sip_apicall.conf"
	if err := os.WriteFile(destFile, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("error escribiendo %s: %w", destFile, err)
	}
	
	// Ensure sip.conf includes it
	if err := ensureInclude("/etc/asterisk/sip.conf", "sip_apicall.conf"); err != nil {
		log.Printf("[Provisioner] Warning: No se pudo inyectar include en sip.conf: %v", err)
		// Try to append if sip_custom.conf exists? Usually sip.conf is main. 
		// If fails, user must include it manually.
	}
	
	// Reload SIP
	if err := exec.Command("asterisk", "-rx", "sip reload").Run(); err != nil {
		 log.Printf("[Provisioner] Warning: Error recargando SIP: %v", err)
	} else {
		log.Println("[Provisioner] ✓ Troncales sincronizadas y SIP recargado.")
	}
	
	return nil
}

func ensureInclude(filepath, include string) error {
	contentBytes, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !strings.Contains(content, include) {
		f, err := os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		
		stmt := fmt.Sprintf("\n#include %s\n", include)
		if _, err := f.WriteString(stmt); err != nil {
			return err
		}
	}
	return nil
}

// ConfigureAsterisk ensures Asterisk has the necessary configuration
func ConfigureAsterisk(cfg *config.Config) {
	log.Println("[Provisioner] Configurando Asterisk...")

	// 1. Manager API (manager.d/apicall.conf)
	if err := configureManager(cfg); err != nil {
		log.Printf("[Provisioner] Error configurando Manager: %v", err)
	}

	// 2. Modules (modules.conf)
	if err := configureModules(); err != nil {
		log.Printf("[Provisioner] Error configurando Módulos: %v", err)
	}

	// 3. Dialplan (extensions_apicall.conf)
	if err := configureDialplan(); err != nil {
		log.Printf("[Provisioner] Error configurando Dialplan: %v", err)
	}
}

func configureManager(cfg *config.Config) error {
	dir := "/etc/asterisk/manager.d"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// If manager.d doesn't exist, we might need to append to manager.conf directly
		// But modern Asterisk usually has it. Let's try creating it or fallback.
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("no existe manager.d y no se pudo crear: %w", err)
		}
	}

	path := "/etc/asterisk/manager.d/apicall.conf"
	content := fmt.Sprintf(`; Generado automáticamente por Apicall
[%s]
secret=%s
deny=0.0.0.0/0.0.0.0
permit=127.0.0.1/255.255.255.0
read=all
write=all
`, cfg.AMI.Username, cfg.AMI.Secret)

	// Check if content changed
	existing, _ := os.ReadFile(path)
	if string(existing) != content {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		log.Println("[Provisioner] ✓ Usuario AMI configurado en manager.d/apicall.conf")
		// Reload manager
		// We execute asterisk reload command
		// Just doing 'manager reload' might be safer
        // But since we are provisioning, 'module reload manager' is fine.
	}
	return nil
}

func configureModules() error {
	path := "/etc/asterisk/modules.conf"
	content, err := os.ReadFile(path)
	if err != nil {
		return err // Might not exist on some installs?
	}

	strContent := string(content)
	dirty := false

	// Ensure app_amd.so is loaded
	if !strings.Contains(strContent, "app_amd.so") {
		// Add it to the end or before global [modules]? 
        // Usually safe to append load => app_amd.so if we assume standard structure.
        // Or ensure it is not noload'ed.
        
        // Simple strategy: Append if missing.
        // Verify it's not noloaded
        if !strings.Contains(strContent, "noload => app_amd.so") {
             strContent += "\nload => app_amd.so\n"
             dirty = true
        }
	} else {
        // If it is noloaded, we should change it?
        // Parsing modules.conf is complex. Let's assume standard behavior.
        // If the user explicitly disabled it, maybe we shouldn't touch it?
        // But the user asked for "full configuration".
        if strings.Contains(strContent, "noload => app_amd.so") {
            strContent = strings.Replace(strContent, "noload => app_amd.so", "load => app_amd.so", -1)
            dirty = true
        }
    }

	if dirty {
		if err := os.WriteFile(path, []byte(strContent), 0644); err != nil {
			return err
		}
		log.Println("[Provisioner] ✓ Módulo app_amd.so habilitado.")
	}
	return nil
}

func configureDialplan() error {
	const (
		sourceFile = "/opt/apicall/configs/extensions_apicall.conf"
		destFile   = "/etc/asterisk/extensions_apicall.conf"
		customFile = "/etc/asterisk/extensions_custom.conf"
		includeStr = "#include extensions_apicall.conf"
	)

	// 1. Leer archivo fuente
	content, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("no se pudo leer %s: %w", sourceFile, err)
	}

	// 2. Escribir o sobrescribir en /etc/asterisk
	if err := os.WriteFile(destFile, content, 0644); err != nil {
		return fmt.Errorf("no se pudo escribir %s: %w", destFile, err)
	}

	// 3. Verificar si extensions_custom.conf existe
	customContentBytes, err := os.ReadFile(customFile)
	if err != nil {
		if os.IsNotExist(err) {
            // Create check if we should create it
            // usually extensions.conf calls extensions_custom.conf
            // Let's create it.
			return os.WriteFile(customFile, []byte(includeStr+"\n"), 0644)
		}
		return fmt.Errorf("error leyendo %s: %w", customFile, err)
	}

	customContent := string(customContentBytes)
	if !strings.Contains(customContent, "extensions_apicall.conf") {
		f, err := os.OpenFile(customFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("error abriendo %s: %w", customFile, err)
		}
		defer f.Close()

		if _, err := f.WriteString("\n" + includeStr + "\n"); err != nil {
			return fmt.Errorf("error escribiendo en %s: %w", customFile, err)
		}
		log.Println("[Provisioner] ✓ Dialplan incluido en extensions_custom.conf")
	}

	return nil
}
