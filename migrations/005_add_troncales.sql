CREATE TABLE IF NOT EXISTS apicall_troncales (
    id INT AUTO_INCREMENT PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL UNIQUE,
    host VARCHAR(255) NOT NULL,
    puerto INT DEFAULT 5060,
    usuario VARCHAR(100),
    password VARCHAR(100),
    contexto VARCHAR(100) DEFAULT 'apicall_context',
    caller_id VARCHAR(100),
    activo BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
