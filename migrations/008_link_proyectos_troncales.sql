-- Tabla de relación N:M entre Proyectos y Troncales
USE apicall_db;

CREATE TABLE IF NOT EXISTS apicall_proyecto_troncal (
    proyecto_id INT NOT NULL,
    troncal_id INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (proyecto_id, troncal_id),
    FOREIGN KEY (proyecto_id) REFERENCES apicall_proyectos(id) ON DELETE CASCADE,
    FOREIGN KEY (troncal_id) REFERENCES apicall_troncales(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Insertar troncal 'sbc233' si no existe (para pruebas)
INSERT INTO apicall_troncales (nombre, host, activo) 
VALUES ('sbc233', '209.38.233.46', TRUE)
ON DUPLICATE KEY UPDATE nombre=nombre;

-- Migración de ejemplo: Asignar todas las troncales existentes al proyecto 937 (opcional/manual)
-- INSERT IGNORE INTO apicall_proyecto_troncal (proyecto_id, troncal_id)
-- SELECT 937, id FROM apicall_troncales;
