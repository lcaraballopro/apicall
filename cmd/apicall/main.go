package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/tabwriter"

	"apicall/internal/ami"
	"apicall/internal/api"
	"apicall/internal/asterisk"
	"apicall/internal/campaign"
	"apicall/internal/config"
	"apicall/internal/database"
	"apicall/internal/dialer"
	"apicall/internal/fastagi"
	"apicall/internal/provisioning"
	"apicall/internal/smartcid"
)

const defaultConfigPath = "/etc/apicall/apicall.yaml"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "start":
		cmdStart()
	case "proyecto":
		cmdProyecto()
	case "status":
		cmdStatus()
	case "troncal":
		cmdTroncal()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Comando desconocido: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Microservicio Apicall - Gestión de Llamadas IVR")
	fmt.Println()
	fmt.Println("Uso:")
	fmt.Println("  apicall start                    Inicia el servicio completo")
	fmt.Println("  apicall proyecto add <args>      Crea un nuevo proyecto")
	fmt.Println("  apicall proyecto list            Lista todos los proyectos")
	fmt.Println("  apicall proyecto delete <id>     Elimina un proyecto")
	fmt.Println("  apicall troncal add <args>       Crea una nueva troncal SIP")
	fmt.Println("  apicall troncal list             Lista las troncales SIP")
	fmt.Println("  apicall troncal delete <id>      Elimina una troncal")
	fmt.Println("  apicall status                   Muestra estado del servicio")
	fmt.Println()
}

// cmdStart inicia todos los servicios
func cmdStart() {
	log.Println("[Main] Apicall Service v1.0")
	log.Println("[Main] Iniciando servicios...")

	// Cargar configuración
	configPath := os.Getenv("APICALL_CONFIG")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("[Main] Error cargando configuración: %v", err)
	}

	// Auto-provisioning (Ensure DB and Asterisk exist)
	provisioning.EnsureInfrastructure(cfg)

	// Conectar a base de datos
	dbConn, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("[Main] Error conectando a base de datos: %v", err)
	}
	defer dbConn.Close()

	repo := database.NewRepository(dbConn)
	log.Println("[Main] ✓ Base de datos conectada")

	// Iniciar cliente AMI
	amiClient := ami.NewClient(&cfg.AMI)
	if err := amiClient.Connect(); err != nil {
		log.Fatalf("[Main] Error conectando AMI: %v", err)
	}
	defer amiClient.Close()
	log.Println("[Main] ✓ Cliente AMI conectado")

	// Inicializar Core Dialer Components
	// ----------------------------------
	
	// 1. Channel Pool (Límites)
	maxChannels := 50
	maxPerTrunk := 20
	if val, err := repo.GetConfig("max_channels"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil && v > 0 {
			maxChannels = v
		}
	}
	if val, err := repo.GetConfig("max_per_trunk"); err == nil && val != "" {
		if v, err := strconv.Atoi(val); err == nil && v > 0 {
			maxPerTrunk = v
		}
	}
	pool := dialer.NewChannelPool(maxChannels, maxPerTrunk)
	log.Printf("[Main] Channel Pool initialized (Global: %d, Trunk: %d)", maxChannels, maxPerTrunk)

	// 2. Active Call Tracker (Memoria)
	tracker := dialer.NewActiveCallTracker()

	// 3. Call Manager (Adapter for AMI Handler)
	callManager := dialer.NewCallManager(pool, tracker)

	// 4. AMI Dialer (Synchronous Originate)
	amiDialer := dialer.NewAMIDialer(amiClient, pool, tracker, repo)
	
	// Configure Smart Caller ID Generator
	if dbConn.DB != nil {
		scidGen := smartcid.NewGenerator(dbConn.DB)
		amiDialer.SetSmartCIDGenerator(scidGen)
	}
	
	amiDialer.Start() // Inicia listener de eventos
	defer amiDialer.Stop()

	// Iniciar AMI Call Status Handler (Tracking & Release)
	// Usamos callManager que implementa la interfaz requerida
	amiHandler := ami.NewCallStatusHandler(amiClient, repo, callManager)
	amiHandler.Start()
	defer amiHandler.Stop()
	log.Println("[Main] ✓ AMI Call Status Handler iniciado")

	// Iniciar servidor FastAGI
	agiServer := fastagi.NewServer(cfg, repo)
	if err := agiServer.Start(); err != nil {
		log.Fatalf("[Main] Error iniciando FastAGI: %v", err)
	}
	log.Println("[Main] ✓ Servidor FastAGI iniciado")

	// Iniciar Worker de Spool (Legacy/Manual Calls)
	asterisk.StartWorker(cfg.Asterisk.MaxCPS, repo, pool, tracker)
	log.Println("[Main] ✓ Worker de Asterisk iniciado")

	// Iniciar API REST
	apiServer := api.NewServer(cfg, repo, amiClient)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("[Main] Error iniciando API: %v", err)
		}
	}()

	log.Println("[Main] ✓ Servidor API REST iniciado")

	// Iniciar Campaign Sweeper Worker
	// Ahora usa AMIDialer directamente
	sweeper := campaign.NewSweeper(repo, amiDialer)
	sweeper.Start()
	defer sweeper.Stop()
	log.Println("[Main] ✓ Campaign Sweeper iniciado")

	// Iniciar Orphan Call Cleaner (limpia llamadas huérfanas en DIALING)
	orphanCleaner := database.NewOrphanCallCleaner(repo)
	orphanCleaner.Start()
	defer orphanCleaner.Stop()
	log.Println("[Main] ✓ Orphan Call Cleaner iniciado")

	log.Println("[Main] ========================================")
	log.Printf("[Main] FastAGI escuchando en %s", cfg.FastAGI.Address())
	log.Printf("[Main] API REST escuchando en %s", cfg.API.Address())
	log.Println("[Main] Servicio iniciado correctamente")
	log.Println("[Main] Presiona Ctrl+C para detener")
	log.Println("[Main] ========================================")

	// Esperar señal de terminación
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Main] Deteniendo servicio...")
	repo.Close()
}

// cmdProyecto gestiona proyectos
func cmdProyecto() {
	if len(os.Args) < 3 {
		fmt.Println("Uso:")
		fmt.Println("  apicall proyecto add --id <id> --nombre <nombre> --caller-id <cid> ...")
		fmt.Println("  apicall proyecto list")
		fmt.Println("  apicall proyecto delete <id>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	// Cargar configuración y conectar a DB
	configPath := os.Getenv("APICALL_CONFIG")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Error cargando configuración: %v", err)
	}

	dbConn, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Error conectando a base de datos: %v", err)
	}
	defer dbConn.Close()

	repo := database.NewRepository(dbConn)

	switch subcommand {
	case "add":
		cmdProyectoAdd(repo)
	case "list":
		cmdProyectoList(repo)
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Uso: apicall proyecto delete <id>")
			os.Exit(1)
		}
		id, _ := strconv.Atoi(os.Args[3])
		cmdProyectoDelete(repo, id)
	default:
		fmt.Printf("Subcomando desconocido: %s\n", subcommand)
		os.Exit(1)
	}
}

// cmdProyectoAdd crea un nuevo proyecto
func cmdProyectoAdd(repo *database.Repository) {
	// Parseo simple de argumentos (en producción usar una librería como cobra/flag)
	proyecto := &database.Proyecto{}

	for i := 3; i < len(os.Args); i += 2 {
		if i+1 >= len(os.Args) {
			break
		}

		key := os.Args[i]
		value := os.Args[i+1]

		switch key {
		case "--id":
			proyecto.ID, _ = strconv.Atoi(value)
		case "--nombre":
			proyecto.Nombre = value
		case "--caller-id":
			proyecto.CallerID = value
		case "--audio":
			proyecto.Audio = value
		case "--dtmf":
			proyecto.DTMFEsperado = value
		case "--desborde":
			proyecto.NumeroDesborde = value
		case "--troncal":
			proyecto.TroncalSalida = value
		case "--prefijo":
			proyecto.PrefijoSalida = value
		case "--ips":
			proyecto.IPsAutorizadas = value
		case "--max-retries":
			proyecto.MaxRetries, _ = strconv.Atoi(value)
		case "--retry-time":
			proyecto.RetryTime, _ = strconv.Atoi(value)
		case "--amd":
			proyecto.AMDActive, _ = strconv.ParseBool(value)
		case "--smart-cid":
			proyecto.SmartCIDActive, _ = strconv.ParseBool(value)
		}
	}

	if proyecto.ID == 0 || proyecto.Nombre == "" {
		fmt.Println("Error: --id y --nombre son requeridos")
		os.Exit(1)
	}

	if err := repo.CreateProyecto(proyecto); err != nil {
		fmt.Printf("Error creando proyecto: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Proyecto #%d '%s' creado correctamente (AMD: %v)\n", proyecto.ID, proyecto.Nombre, proyecto.AMDActive)
}

// cmdProyectoList lista todos los proyectos
func cmdProyectoList(repo *database.Repository) {
	proyectos, err := repo.ListProyectos()
	if err != nil {
		fmt.Printf("Error listando proyectos: %v\n", err)
		os.Exit(1)
	}

	if len(proyectos) == 0 {
		fmt.Println("No hay proyectos configurados")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNOMBRE\tCALLER ID\tAUDIO\tAMD\tSMART\tDESBORDE\tTRONCAL\tRETRIES\tWAIT")
	fmt.Fprintln(w, "---\t------\t---------\t-----\t---\t-----\t--------\t-------\t-------\t----")

	for _, p := range proyectos {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%v\t%v\t%s\t%s\t%d\t%d\n",
			p.ID, p.Nombre, p.CallerID, p.Audio, p.AMDActive, p.SmartCIDActive,
			p.NumeroDesborde, p.TroncalSalida, p.MaxRetries, p.RetryTime)
	}

	w.Flush()
}

// cmdProyectoDelete elimina un proyecto
func cmdProyectoDelete(repo *database.Repository, id int) {
	if err := repo.DeleteProyecto(id); err != nil {
		fmt.Printf("Error eliminando proyecto: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Proyecto #%d eliminado\n", id)
}

// cmdStatus muestra el estado del servicio
func cmdStatus() {
	fmt.Println("Apicall Service Status")
	fmt.Println("======================")
	fmt.Println()
	fmt.Println("Para verificar el estado del servicio:")
	fmt.Println("  systemctl status apicall")
	fmt.Println()
	fmt.Println("Para ver logs en tiempo real:")
	fmt.Println("  journalctl -u apicall -f")
	fmt.Println()
	fmt.Println("Para verificar conectividad FastAGI:")
	fmt.Println("  nc -zv 127.0.0.1 4573")
	fmt.Println()
	fmt.Println("Para verificar API REST:")
	fmt.Println("  curl http://localhost:8080/health")
}

// cmdTroncal gestiona troncales
func cmdTroncal() {
	if len(os.Args) < 3 {
		fmt.Println("Uso:")
		fmt.Println("  apicall troncal add --nombre <n> --host <h> --user <u> --pass <p> ...")
		fmt.Println("  apicall troncal list")
		fmt.Println("  apicall troncal delete <id>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	// Config + DB
	configPath := os.Getenv("APICALL_CONFIG")
	if configPath == "" {
		configPath = defaultConfigPath
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Error config: %v", err)
	}
	dbConn, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Error DB: %v", err)
	}
	defer dbConn.Close()
	repo := database.NewRepository(dbConn)

	switch subcommand {
	case "add":
		cmdTroncalAdd(repo, cfg)
	case "list":
		cmdTroncalList(repo)
	case "delete":
		cmdTroncalDelete(repo, idAtoi(os.Args[3]), cfg)
	default:
		fmt.Printf("Subcomando desconocido: %s\n", subcommand)
		os.Exit(1)
	}
}

func idAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func cmdTroncalAdd(repo *database.Repository, cfg *config.Config) {
	t := &database.Troncal{Puerto: 5060, Contexto: "apicall_context", Activo: true}
	
	for i := 3; i < len(os.Args); i += 2 {
		if i+1 >= len(os.Args) { break }
		key, val := os.Args[i], os.Args[i+1]
		switch key {
		case "--nombre": t.Nombre = val
		case "--host": t.Host = val
		case "--port": t.Puerto, _ = strconv.Atoi(val)
		case "--user": t.Usuario = val
		case "--pass": t.Password = val
		case "--context": t.Contexto = val
		case "--cid": t.CallerID = val
		}
	}

	if t.Nombre == "" || t.Host == "" {
		fmt.Println("Error: --nombre y --host son requeridos")
		os.Exit(1)
	}

	if err := repo.CreateTroncal(t); err != nil {
		fmt.Printf("Error creando troncal: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✓ Troncal '%s' agregada en DB.\n", t.Nombre)
	
	// Sync force
	if err := provisioning.SyncTroncales(repo); err != nil {
		fmt.Printf("Warning: Error sincronizando con Asterisk: %v\n", err)
	}
}

func cmdTroncalList(repo *database.Repository) {
	ts, err := repo.ListTroncales()
	if err != nil {
		log.Fatal(err)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNOMBRE\tHOST\tUSER\tCONTEXT\tACTIVO")
	fmt.Fprintln(w, "---\t------\t----\t----\t-------\t------")
	for _, t := range ts {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%v\n", t.ID, t.Nombre, t.Host, t.Usuario, t.Contexto, t.Activo)
	}
	w.Flush()
}

func cmdTroncalDelete(repo *database.Repository, id int, cfg *config.Config) {
	if err := repo.DeleteTroncal(id); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Troncal #%d eliminada.\n", id)
	provisioning.SyncTroncales(repo)
}
