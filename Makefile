.PHONY: build clean install run test

# Variables
BINARY_NAME=apicall
BINARY_DIR=bin
INSTALL_DIR=/usr/local/bin

# Compilar el binario
build:
	@echo "Compilando $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/apicall
	@echo "Binario creado en $(BINARY_DIR)/$(BINARY_NAME)"

# Compilar y ejecutar
run: build
	@./$(BINARY_DIR)/$(BINARY_NAME) start

# Instalar binario en el sistema
install: build
	@echo "Instalando $(BINARY_NAME) en $(INSTALL_DIR)..."
	@cp $(BINARY_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Instalado correctamente"

# Limpiar archivos generados
clean:
	@echo "Limpiando..."
	@rm -rf $(BINARY_DIR)
	@go clean

# Ejecutar tests
test:
	@go test -v ./...

# Descargar dependencias
deps:
	@echo "Descargando dependencias..."
	@go mod download
	@go mod tidy

# Compilar para producción (optimizado)
build-prod:
	@echo "Compilando para producción..."
	@mkdir -p $(BINARY_DIR)
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/apicall
	@echo "Binario optimizado creado"
