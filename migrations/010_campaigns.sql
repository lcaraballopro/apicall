-- Migración 010: Gestión de Campañas Masivas (CSV)
-- Permite subir CSV con números y asignar schedules por día

-- Tabla de campañas masivas
CREATE TABLE IF NOT EXISTS apicall_campaigns (
    id INT AUTO_INCREMENT PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    proyecto_id INT NOT NULL,
    estado ENUM('draft', 'active', 'paused', 'completed', 'stopped') DEFAULT 'draft',
    total_contactos INT DEFAULT 0,
    contactos_procesados INT DEFAULT 0,
    contactos_exitosos INT DEFAULT 0,
    contactos_fallidos INT DEFAULT 0,
    fecha_inicio DATETIME NULL,
    fecha_fin DATETIME NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (proyecto_id) REFERENCES apicall_proyectos(id) ON DELETE CASCADE,
    INDEX idx_proyecto (proyecto_id),
    INDEX idx_estado (estado)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Tabla de contactos de campaña (números del CSV)
CREATE TABLE IF NOT EXISTS apicall_campaign_contacts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    campaign_id INT NOT NULL,
    telefono VARCHAR(20) NOT NULL,
    datos_adicionales JSON NULL COMMENT 'Columnas adicionales del CSV como JSON',
    estado ENUM('pending', 'dialing', 'completed', 'failed', 'skipped') DEFAULT 'pending',
    intentos INT DEFAULT 0,
    ultimo_intento DATETIME NULL,
    resultado VARCHAR(50) NULL COMMENT 'Status final de la llamada',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (campaign_id) REFERENCES apicall_campaigns(id) ON DELETE CASCADE,
    INDEX idx_campaign (campaign_id),
    INDEX idx_estado (estado),
    INDEX idx_telefono (telefono)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Tabla de schedules (horarios por día de la semana)
CREATE TABLE IF NOT EXISTS apicall_campaign_schedules (
    id INT AUTO_INCREMENT PRIMARY KEY,
    campaign_id INT NOT NULL,
    dia_semana TINYINT NOT NULL COMMENT '0=Domingo, 1=Lunes, ..., 6=Sábado',
    hora_inicio TIME NOT NULL COMMENT 'Hora de inicio (ej: 09:00:00)',
    hora_fin TIME NOT NULL COMMENT 'Hora de fin (ej: 18:00:00)',
    activo BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (campaign_id) REFERENCES apicall_campaigns(id) ON DELETE CASCADE,
    UNIQUE KEY unique_schedule (campaign_id, dia_semana),
    INDEX idx_campaign (campaign_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
