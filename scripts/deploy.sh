#!/bin/bash
# script: scripts/deploy.sh
# Descripción: Script automatizado para desplegar cambios en producción.
# Uso: ./scripts/deploy.sh [opciones]

set -e # Detener script si hay errores

# Colores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Iniciando Despliegue de Apicall ===${NC}"

# 1. Actualizar Código
echo -e "${YELLOW}[1/3] Actualizando repositorio...${NC}"
git pull origin main || { echo -e "${RED}Error al actualizar git${NC}"; exit 1; }

# Detección de entorno
MODE="auto"
if [[ "$1" == "--docker" ]]; then MODE="docker"; fi
if [[ "$1" == "--systemd" ]]; then MODE="systemd"; fi

if [[ "$MODE" == "auto" ]]; then
    if [[ -f "docker-compose.yml" ]] && docker compose version &> /dev/null; then
        MODE="docker"
    elif [[ -f "/etc/systemd/system/apicall.service" ]]; then
        MODE="systemd"
    else
        echo -e "${RED}No se pudo detectar el entorno (Docker o Systemd).${NC}"
        echo "Usa: ./scripts/deploy.sh --docker O ./scripts/deploy.sh --systemd"
        exit 1
    fi
fi

# 2. Ejecutar Despliegue según modo
echo -e "${YELLOW}[2/3] Modo detectado: ${MODE}${NC}"

if [[ "$MODE" == "docker" ]]; then
    echo "Reconstruyendo contenedores..."
    docker-compose up -d --build apicall
    echo "Limpiando imágenes antiguas..."
    docker image prune -f
elif [[ "$MODE" == "systemd" ]]; then
    echo "Compilando binario..."
    make build-prod
    echo "Instalando binario..."
    sudo make install
    echo "Reiniciando servicio systemd..."
    sudo systemctl restart apicall
fi

# 3. Verificación
echo -e "${YELLOW}[3/3] Verificando estado...${NC}"
sleep 2

if [[ "$MODE" == "docker" ]]; then
    docker-compose ps | grep apicall
elif [[ "$MODE" == "systemd" ]]; then
    systemctl status apicall --no-pager
fi

echo -e "${GREEN}=== Despliegue Completado con Éxito ===${NC}"
