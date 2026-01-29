#!/bin/bash
#
# Script de Pruebas de Estrés para Apicall
# Prueba la capacidad del sistema generando llamadas masivas
#

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuración por defecto
API_URL="http://localhost:8080/api/v1/call"
PROYECTO_ID=937
NUM_CALLS=100
COUNTRY="colombia"  # mexico, colombia
CLEAN_BEFORE=false
WAIT_AFTER=5

# Banner
echo -e "${BLUE}"
echo "╔═══════════════════════════════════════════╗"
echo "║   APICALL - PRUEBA DE ESTRÉS             ║"
echo "╔═══════════════════════════════════════════╗"
echo -e "${NC}"

# Función de ayuda
show_help() {
    cat << EOF
Uso: $0 [opciones]

Opciones:
    -n, --calls NUM         Número de llamadas a generar (default: 100)
    -p, --proyecto ID       ID del proyecto (default: 937)
    -c, --country PAIS      País: mexico, colombia (default: colombia)
    -u, --url URL           URL de la API (default: http://localhost:8080/api/v1/call)
    -w, --wait SEGUNDOS     Tiempo de espera post-test (default: 5)
    --clean                 Limpiar cola y logs antes del test
    -h, --help              Mostrar esta ayuda

Ejemplos:
    # 100 llamadas a Colombia
    $0 -n 100 -c colombia

    # 500 llamadas a México con limpieza previa
    $0 -n 500 -c mexico --clean

    # Test masivo 1000 llamadas
    $0 -n 1000 --clean -w 10

EOF
}

# Parsear argumentos
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--calls)
            NUM_CALLS="$2"
            shift 2
            ;;
        -p|--proyecto)
            PROYECTO_ID="$2"
            shift 2
            ;;
        -c|--country)
            COUNTRY="$2"
            shift 2
            ;;
        -u|--url)
            API_URL="$2"
            shift 2
            ;;
        -w|--wait)
            WAIT_AFTER="$2"
            shift 2
            ;;
        --clean)
            CLEAN_BEFORE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Opción desconocida: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Función para generar número aleatorio según país
generate_phone() {
    case $COUNTRY in
        mexico)
            # Formato: 52 + LADA (2 dígitos) + 8 dígitos
            LADAS=("55" "81" "33" "56" "22" "664" "686")
            LADA=${LADAS[$RANDOM % ${#LADAS[@]}]}
            SUFFIX=$(shuf -i 10000000-99999999 -n 1)
            echo "52${LADA}${SUFFIX}"
            ;;
        colombia)
            # Formato: 57 + prefijo (3) + 7 dígitos
            PREFIX="319"
            SUFFIX=$(shuf -i 1000000-9999999 -n 1)
            echo "57${PREFIX}${SUFFIX}"
            ;;
        *)
            echo -e "${RED}País no soportado: $COUNTRY${NC}"
            exit 1
            ;;
    esac
}

# Limpieza previa si se solicita
if [ "$CLEAN_BEFORE" = true ]; then
    echo -e "${YELLOW}🧹 Limpiando sistema...${NC}"
    sudo rm -f /var/spool/asterisk/outgoing/*.call 2>/dev/null || true
    mysql -h 127.0.0.1 -P 3307 -u apicall -papicall_pass apicall_db \
        -e "DELETE FROM apicall_call_log WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR)" 2>/dev/null || true
    echo -e "${GREEN}✓ Sistema limpio${NC}"
    echo
fi

# Información del test
echo -e "${BLUE}═══ CONFIGURACIÓN DEL TEST ═══${NC}"
echo -e "  Llamadas:    ${GREEN}${NUM_CALLS}${NC}"
echo -e "  Proyecto:    ${GREEN}${PROYECTO_ID}${NC}"
echo -e "  País:        ${GREEN}${COUNTRY}${NC}"
echo -e "  API URL:     ${GREEN}${API_URL}${NC}"
echo

# Iniciar prueba
echo -e "${BLUE}🚀 INICIANDO TEST: $(date +%H:%M:%S.%N)${NC}"
START_TIME=$(date +%s.%N)

# Generar llamadas en paralelo
for i in $(seq 1 $NUM_CALLS); do
    PHONE=$(generate_phone)
    curl -s -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -d "{\"proyecto_id\":$PROYECTO_ID,\"telefono\":\"$PHONE\"}" \
        > /dev/null &
    
    # Mostrar progreso cada 50 llamadas
    if (( i % 50 == 0 )); then
        echo -e "${YELLOW}  → $i/$NUM_CALLS llamadas enviadas...${NC}"
    fi
done

# Esperar a que todas terminen
wait

END_TIME=$(date +%s.%N)
DURATION=$(echo "$END_TIME - $START_TIME" | bc)

echo -e "${GREEN}✅ ENCOLADO COMPLETO: $(date +%H:%M:%S.%N)${NC}"
echo -e "${GREEN}   Tiempo total: ${DURATION} segundos${NC}"
echo

# Métricas inmediatas
echo -e "${BLUE}═══ MÉTRICAS POST-ENCOLADO ═══${NC}"
mysql -h 127.0.0.1 -P 3307 -u apicall -papicall_pass apicall_db << EOF
SELECT 
    COUNT(*) as 'Total Encoladas',
    COUNT(DISTINCT caller_id_used) as 'Caller IDs Únicos',
    COUNT(DISTINCT telefono) as 'Números Únicos'
FROM apicall_call_log 
WHERE created_at > DATE_SUB(NOW(), INTERVAL 2 MINUTE);
EOF

SPOOL_COUNT=$(ls /var/spool/asterisk/outgoing/*.call 2>/dev/null | wc -l)
echo -e "${YELLOW}Archivos en spool: ${SPOOL_COUNT}${NC}"
echo

# Ejemplos de Caller IDs generados
echo -e "${BLUE}═══ EJEMPLOS DE CALLER IDs GENERADOS ═══${NC}"
mysql -h 127.0.0.1 -P 3307 -u apicall -papicall_pass apicall_db << EOF
SELECT telefono, caller_id_used 
FROM apicall_call_log 
WHERE created_at > DATE_SUB(NOW(), INTERVAL 2 MINUTE) 
LIMIT 10;
EOF
echo

# Esperar y mostrar progreso de procesamiento
if [ "$WAIT_AFTER" -gt 0 ]; then
    echo -e "${YELLOW}⏰ Monitoreando procesamiento durante ${WAIT_AFTER} segundos...${NC}"
    
    SPOOL_BEFORE=$SPOOL_COUNT
    START_MONITOR=$(date +%s)
    sleep $WAIT_AFTER
    END_MONITOR=$(date +%s)
    
    SPOOL_AFTER=$(ls /var/spool/asterisk/outgoing/*.call 2>/dev/null | wc -l)
    PROCESSED=$((SPOOL_BEFORE - SPOOL_AFTER))
    ACTUAL_WAIT=$((END_MONITOR - START_MONITOR))
    
    echo -e "${GREEN}═══ PROGRESO DE PROCESAMIENTO ═══${NC}"
    echo -e "  Antes:       ${SPOOL_BEFORE} archivos"
    echo -e "  Ahora:       ${SPOOL_AFTER} archivos"
    echo -e "  Procesados:  ${GREEN}${PROCESSED}${NC} en ${ACTUAL_WAIT}s"
    
    if [ $ACTUAL_WAIT -gt 0 ] && [ $PROCESSED -gt 0 ]; then
        RATE=$(echo "scale=2; $PROCESSED / $ACTUAL_WAIT" | bc)
        echo -e "  Rate:        ${GREEN}${RATE} CPS${NC}"
    fi
    
    # Si quedan archivos, mostrar tiempo estimado
    if [ $SPOOL_AFTER -gt 0 ] && [ $PROCESSED -gt 0 ]; then
        ETA=$(echo "scale=1; $SPOOL_AFTER / $RATE" | bc 2>/dev/null)
        echo -e "  ETA restante: ${YELLOW}~${ETA}s${NC} (${SPOOL_AFTER} archivos pendientes)"
    fi
fi

# Resumen final
echo
echo -e "${BLUE}╔═══════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║          TEST COMPLETADO ✓                ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════╝${NC}"
echo -e "  ${GREEN}$NUM_CALLS llamadas en $DURATION segundos${NC}"
echo -e "  ${GREEN}Throughput: $(echo "scale=2; $NUM_CALLS / $DURATION" | bc) req/s${NC}"
echo
