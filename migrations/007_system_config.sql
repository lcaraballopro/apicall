-- Sistema de configuración dinámica para apicall
-- Permite configurar CPS y otros parámetros desde la base de datos

USE apicall_db;

-- Tabla de configuraciones del sistema
CREATE TABLE IF NOT EXISTS apicall_config (
    id INT AUTO_INCREMENT PRIMARY KEY,
    config_key VARCHAR(50) NOT NULL UNIQUE COMMENT 'Clave de configuración',
    config_value VARCHAR(255) NOT NULL COMMENT 'Valor de la configuración',
    description VARCHAR(255) COMMENT 'Descripción del parámetro',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_config_key (config_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Insertar configuraciones por defecto
INSERT INTO apicall_config (config_key, config_value, description) VALUES
    ('max_cps', '50', 'Máximo de llamadas por segundo (CPS)'),
    ('ami_buffer_size', '10000', 'Tamaño del buffer de eventos AMI'),
    ('queue_size', '10000', 'Tamaño máximo de la cola de llamadas'),
    ('default_max_retries', '3', 'Reintentos por defecto si no está definido en el proyecto'),
    ('default_retry_time', '300', 'Tiempo entre reintentos en segundos')
ON DUPLICATE KEY UPDATE config_value = VALUES(config_value);
