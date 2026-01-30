-- Per-project blacklist table
-- Migration: 011_blacklist.sql
-- Allows blocking specific phone numbers from being called within each project

CREATE TABLE IF NOT EXISTS apicall_blacklist (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    proyecto_id INT NOT NULL,
    telefono VARCHAR(20) NOT NULL,
    razon VARCHAR(100) DEFAULT NULL COMMENT 'Razón opcional del bloqueo',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_proyecto_telefono (proyecto_id, telefono),
    INDEX idx_proyecto (proyecto_id),
    INDEX idx_telefono (telefono),
    FOREIGN KEY (proyecto_id) REFERENCES apicall_proyectos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
