CREATE TABLE IF NOT EXISTS apicall_callerid_stats (
    prefix VARCHAR(10) NOT NULL,
    pattern VARCHAR(20) NOT NULL,
    attempts INT DEFAULT 0,
    answers INT DEFAULT 0,
    score FLOAT DEFAULT 0.0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (prefix, pattern)
);

ALTER TABLE apicall_proyectos ADD COLUMN IF NOT EXISTS smart_cid_active BOOLEAN DEFAULT FALSE;
ALTER TABLE apicall_call_log ADD COLUMN IF NOT EXISTS caller_id_used VARCHAR(20);
