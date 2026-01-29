# Guía de Despliegue de Apicall

Esta guía detalla los pasos para desplegar, actualizar y mantener el microservicio `apicall` en un entorno de producción.

## Requisitos Previos

*   **Sistema Operativo**: Linux (Debian 11+, Ubuntu 20.04+, CentOS 8+).
*   **Base de Datos**: PostgreSQL 14+ (o MariaDB compatible).
*   **Telefonía**: Asterisk 16+ instalado y configurado.
*   **Herramientas**:
    *   `git`
    *   `make` (para instalación manual)
    *   `go` (Go 1.21+ para compilación manual)
    *   `docker` y `docker-compose` (opcional, para despliegue en contenedores)

---

## 🐳 Opción 1: Despliegue con Docker (Recomendado)

Usar Docker facilita la gestión de dependencias y asegura un entorno consistente.

### 1. Instalación Inicial

1.  **Clonar el repositorio**:
    ```bash
    git clone <url-del-repo> /opt/apicall
    cd /opt/apicall
    ```

2.  **Configurar Variables**:
    Edita el archivo `docker-compose.yml` o crea un archivo `.env` para ajustar las credenciales de base de datos y puertos si es necesario. Asegúrate de que los volúmenes apunten a los directorios correctos de tu host (especialmente los sonidos de Asterisk).

3.  **Iniciar Servicios**:
    ```bash
    docker-compose up -d
    ```
    Esto descargará las imágenes, compilará `apicall`, iniciará la base de datos (si está incluida) y levantará el servicio.

4.  **Verificar Estado**:
    ```bash
    docker-compose ps
    docker-compose logs -f apicall
    ```

### 🔄 Cómo Aplicar Cambios (Recompilar)

Si has modificado el código fuente o la configuración dentro de la imagen, sigue estos pasos para actualizar:

1.  **Bajar el código actualizado** (si aplica):
    ```bash
    git pull origin main
    ```

2.  **Reconstruir y Reiniciar**:
    Este comando fuerza la recompilación de la imagen `apicall` y recrea el contenedor:
    ```bash
    docker-compose up -d --build apicall
    ```
    *Nota: `--build` es clave para que Docker detecte cambios en el código y recompile el binario.*

3.  **Limpiar imágenes viejas** (opcional):
    ```bash
    docker image prune -f
    ```

---

## 🛠 Opción 2: Instalación Manual (Systemd)

Este método corre el binario directamente en el host, ofreciendo máximo rendimiento y simplicidad si no usas Docker.

### 1. Compilación e Instalación

1.  **Instalar Go**: Asegúrate de tener Go instalado (`go version`).

2.  **Compilar Binario**:
    Sitúate en el directorio del proyecto y ejecuta:
    ```bash
    make build
    # O para producción (optimizado):
    make build-prod
    ```
    Esto generará el ejecutable en `./bin/apicall`.

3.  **Instalar en el Sistema**:
    ```bash
    # Copia el binario a /usr/local/bin
    sudo make install
    ```

### 2. Configurar como Servicio (Systemd)

1.  **Copiar archivo de servicio**:
    ```bash
    sudo cp configs/apicall.service /etc/systemd/system/
    ```

2.  **Ajustar Configuración**:
    Edita `/etc/systemd/system/apicall.service` si necesitas cambiar rutas o usuario.
    ```bash
    sudo nano /etc/systemd/system/apicall.service
    ```

3.  **Activar Servicio**:
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable apicall
    sudo systemctl start apicall
    ```

4.  **Ver Logs**:
    ```bash
    journalctl -u apicall -f
    ```

### 🔄 Cómo Aplicar Cambios (Recompilar Manualmente)

Si cambias el código Go o la configuración, sigue estos pasos:

1.  **Detener el servicio**:
    ```bash
    sudo systemctl stop apicall
    ```

2.  **Recompilar e Instalar**:
    ```bash
    git pull origin main  # Si hay cambios remotos
    make build-prod       # Compilar nueva versión
    sudo make install     # Sobrescribir binario en /usr/local/bin
    ```

3.  **Reiniciar el servicio**:
    ```bash
    sudo systemctl start apicall
    ```

    *Truco: Puedes hacer todo en una línea:*
    ```bash
    make build-prod && sudo make install && sudo systemctl restart apicall
    ```

---

## 📝 Resumen de Comandos Mágicos

| Acción | Docker | Manual (Systemd) |
| :--- | :--- | :--- |
| **Iniciar** | `docker-compose up -d` | `systemctl start apicall` |
| **Detener** | `docker-compose stop` | `systemctl stop apicall` |
| **Ver Logs** | `docker-compose logs -f --tail=100` | `journalctl -u apicall -f` |
| **Actualizar Código** | `docker-compose up -d --build` | `make build-prod && make install && systemctl restart apicall` |
| **Entrar a consola** | `docker exec -it apicall-server sh` | `bash` (ya estás en el host) |
