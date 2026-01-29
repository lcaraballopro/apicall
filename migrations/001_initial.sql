-- Esquema inicial para apicall_db
CREATE DATABASE IF NOT EXISTS apicall_db;
USE apicall_db;

-- Tabla de proyectos (campañas)
CREATE TABLE IF NOT EXISTS apicall_proyectos (
    id INT PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    caller_id VARCHAR(20) NOT NULL,
    audio VARCHAR(100) NOT NULL COMMENT 'Nombre del archivo .wav a reproducir',
    dtmf_esperado CHAR(1) DEFAULT '1' COMMENT 'Tono que activa el desvío',
    numero_desborde VARCHAR(20) NOT NULL COMMENT 'Teléfono destino para transferencia',
    troncal_salida VARCHAR(50) NOT NULL COMMENT 'Nombre del SBC o troncal',
    prefijo_salida VARCHAR(10) DEFAULT '' COMMENT 'Prefijo técnico de marcación',
    ips_autorizadas TEXT COMMENT 'IPs permitidas para disparar API (separadas por coma)',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Tabla de logs de llamadas
CREATE TABLE IF NOT EXISTS apicall_call_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    proyecto_id INT NOT NULL,
    telefono VARCHAR(20) NOT NULL,
    dtmf_marcado CHAR(1) COMMENT 'Tecla presionada por el cliente',
    interacciono BOOLEAN DEFAULT FALSE COMMENT 'Indica si el cliente respondió al IVR',
    status VARCHAR(20) NOT NULL COMMENT 'Estado final: ANSWER, TRANSFER, NO-DTMF, etc.',
    duracion INT DEFAULT 0 COMMENT 'Duración en segundos',
    uniqueid VARCHAR(50) COMMENT 'Uniqueid de Asterisk',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_proyecto (proyecto_id),
    INDEX idx_telefono (telefono),
    INDEX idx_created (created_at),
    INDEX idx_status (status),
    FOREIGN KEY (proyecto_id) REFERENCES apicall_proyectos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Insertar proyecto de ejemplo
INSERT INTO apicall_proyectos (id, nombre, caller_id, audio, dtmf_esperado, numero_desborde, troncal_salida, prefijo_salida, ips_autorizadas)
VALUES (937, 'Yes', '5551234567', 'bienvenida.wav', '1', '3001234567', 'sbc233', '1122', '127.0.0.1,192.168.1.0/24')
ON DUPLICATE KEY UPDATE nombre=VALUES(nombre);
