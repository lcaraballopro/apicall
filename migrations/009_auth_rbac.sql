-- Tabla de Usuarios
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role ENUM('admin', 'supervisor', 'viewer') DEFAULT 'viewer',
    full_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    active BOOLEAN DEFAULT TRUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Asignación de Usuarios a Proyectos (Para Supervisores)
CREATE TABLE IF NOT EXISTS user_proyectos (
    user_id INT,
    proyecto_id INT,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, proyecto_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (proyecto_id) REFERENCES apicall_proyectos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Usuario Admin por defecto (Pass: admin123 - Se debe cambiar)
-- Hash bcrypt para 'admin123'
INSERT IGNORE INTO users (username, password_hash, role, full_name) 
VALUES ('admin', '$2a$10$Jt4ezuu7HMTzGM1uHqcDauMuQTQrs7V9hx6pCbq5nT.dwWonwBdwa', 'admin', 'System Administrator');
-- Nota: Generaré un hash real en el código Go o usare uno conocido para el admin temporal
