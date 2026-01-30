-- Add timezone column to projects
ALTER TABLE apicall_proyectos ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) DEFAULT 'America/Bogota';
