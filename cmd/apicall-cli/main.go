package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	apiHost string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "apicall-cli",
		Short: "CLI para administrar Apicall",
		Long:  `Una herramienta de línea de comandos para gestionar el microservicio Apicall de forma remota.`,
	}

	rootCmd.PersistentFlags().StringVar(&apiHost, "host", "http://localhost:8080", "URL base de la API (ej: http://209.38.233.46:8080)")

	// === PROYECTOS ===
	var projectCmd = &cobra.Command{
		Use:   "project",
		Short: "Gestionar proyectos",
	}

	var projectListCmd = &cobra.Command{
		Use:   "list",
		Short: "Listar proyectos",
		Run:   runProjectList,
	}

	var projectAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Crear proyecto",
		Run:   runProjectAdd,
	}
	// Flags para project add
	projectAddCmd.Flags().Int("id", 0, "ID del proyecto (requerido)")
	projectAddCmd.Flags().String("nombre", "", "Nombre del proyecto (requerido)")
	projectAddCmd.Flags().String("audio", "", "Archivo de audio")
	projectAddCmd.Flags().String("cid", "", "Caller ID")
	projectAddCmd.Flags().String("trunk", "", "Troncal de salida")
	projectAddCmd.Flags().String("prefix", "", "Prefijo tecnológico")
	projectAddCmd.Flags().String("desborde", "", "Número de desborde")
	projectAddCmd.Flags().Bool("amd", false, "Activar detección de contestadora")
	projectAddCmd.Flags().Bool("smart-cid", false, "Activar Smart Caller ID")

	var projectDeleteCmd = &cobra.Command{
		Use:   "delete [id]",
		Short: "Eliminar proyecto",
		Args:  cobra.ExactArgs(1),
		Run:   runProjectDelete,
	}

	projectCmd.AddCommand(projectListCmd, projectAddCmd, projectDeleteCmd)

	// === TRONCALES ===
	var trunkCmd = &cobra.Command{
		Use:   "trunk",
		Short: "Gestionar troncales SIP",
	}

	var trunkListCmd = &cobra.Command{
		Use:   "list",
		Short: "Listar troncales",
		Run:   runTrunkList,
	}

	var trunkAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Crear troncal SIP",
		Run:   runTrunkAdd,
	}
	trunkAddCmd.Flags().String("nombre", "", "Nombre de la troncal")
	trunkAddCmd.Flags().String("host", "", "Host/IP")
	trunkAddCmd.Flags().Int("port", 5060, "Puerto SIP")
	trunkAddCmd.Flags().String("user", "", "Usuario SIP")
	trunkAddCmd.Flags().String("pass", "", "Contraseña SIP")
	trunkAddCmd.Flags().String("context", "apicall_context", "Contexto entrante")

	var trunkDeleteCmd = &cobra.Command{
		Use:   "delete [id]",
		Short: "Eliminar troncal",
		Args:  cobra.ExactArgs(1),
		Run:   runTrunkDelete,
	}

	trunkCmd.AddCommand(trunkListCmd, trunkAddCmd, trunkDeleteCmd)

	// === LLAMADAS ===
	var callCmd = &cobra.Command{
		Use:   "call",
		Short: "Realizar una llamada de prueba",
		Run:   runCall,
	}
	callCmd.Flags().Int("project", 0, "ID del proyecto")
	callCmd.Flags().String("number", "", "Número a marcar")

	// === ROOT ===
	rootCmd.AddCommand(projectCmd, trunkCmd, callCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// --- HANDLERS ---

func runProjectList(cmd *cobra.Command, args []string) {
	url := fmt.Sprintf("%s/api/v1/proyectos", apiHost)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error conectando a API: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("Error API: %s\n", resp.Status)
		return
	}

	var proyectos []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&proyectos)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNOMBRE\tCID\tTRONCAL\tAMD")
	fmt.Fprintln(w, "--\t------\t---\t-------\t---")
	for _, p := range proyectos {
		fmt.Fprintf(w, "%.0f\t%s\t%s\t%s\t%v\n", p["id"], p["nombre"], p["caller_id"], p["troncal_salida"], p["amd_active"])
	}
	w.Flush()
}

func runProjectAdd(cmd *cobra.Command, args []string) {
	id, _ := cmd.Flags().GetInt("id")
	nombre, _ := cmd.Flags().GetString("nombre")
	
	if id == 0 || nombre == "" {
		fmt.Println("Error: --id y --nombre son requeridos")
		return
	}

	body := map[string]interface{}{
		"id":             id,
		"nombre":         nombre,
		"audio":          getString(cmd, "audio"),
		"caller_id":      getString(cmd, "cid"),
		"troncal_salida": getString(cmd, "trunk"),
		"prefijo_salida": getString(cmd, "prefix"),
		"num_desborde":   getString(cmd, "desborde"),
		"amd_active":     getBool(cmd, "amd"),
		"smart_active":   getBool(cmd, "smart-cid"),
	}

	sendPost(fmt.Sprintf("%s/api/v1/proyectos", apiHost), body)
}

func runProjectDelete(cmd *cobra.Command, args []string) {
	id := args[0]
	url := fmt.Sprintf("%s/api/v1/proyectos/delete?id=%s", apiHost, id)
	
	req, _ := http.NewRequest("DELETE", url, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 200 {
		fmt.Printf("Proyecto %s eliminado.\n", id)
	} else {
		fmt.Printf("Error API: %s\n", resp.Status)
	}
}

func runTrunkList(cmd *cobra.Command, args []string) {
	url := fmt.Sprintf("%s/api/v1/troncales", apiHost)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var troncales []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&troncales)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNOMBRE\tHOST\tUSER")
	fmt.Fprintln(w, "--\t------\t----\t----")
	for _, t := range troncales {
		fmt.Fprintf(w, "%.0f\t%s\t%s\t%s\n", t["id"], t["nombre"], t["host"], t["usuario"])
	}
	w.Flush()
}

func runTrunkAdd(cmd *cobra.Command, args []string) {
	body := map[string]interface{}{
		"nombre":   getString(cmd, "nombre"),
		"host":     getString(cmd, "host"),
		"puerto":   getInt(cmd, "port"),
		"usuario":  getString(cmd, "user"),
		"password": getString(cmd, "pass"),
		"contexto": getString(cmd, "context"),
		"activo":   true,
	}
	sendPost(fmt.Sprintf("%s/api/v1/troncales", apiHost), body)
}

func runTrunkDelete(cmd *cobra.Command, args []string) {
	id := args[0]
	url := fmt.Sprintf("%s/api/v1/troncales/delete?id=%s", apiHost, id)
	req, _ := http.NewRequest("DELETE", url, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		fmt.Printf("Troncal %s eliminada.\n", id)
	} else {
		fmt.Printf("Error: %s\n", resp.Status)
	}
}

func runCall(cmd *cobra.Command, args []string) {
	project, _ := cmd.Flags().GetInt("project")
	number, _ := cmd.Flags().GetString("number")

	if project == 0 || number == "" {
		fmt.Println("Error: --project y --number son requeridos")
		return
	}

	body := map[string]interface{}{
		"proyecto_id": project,
		"telefono":    number,
	}
	
	start := time.Now()
	sendPost(fmt.Sprintf("%s/api/v1/call", apiHost), body)
	fmt.Printf("Tiempo: %v\n", time.Since(start))
}

// Helpers
func getString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}
func getInt(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)
	return v
}
func getBool(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}

func sendPost(url string, data interface{}) {
	payload, _ := json.Marshal(data)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error de conexión: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println("Éxito!")
		fmt.Println(string(body))
	} else {
		fmt.Printf("Error (%s): %s\n", resp.Status, string(body))
	}
}
