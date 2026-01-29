ALTER TABLE apicall_proyectos ADD COLUMN max_retries INT DEFAULT 2;
ALTER TABLE apicall_proyectos ADD COLUMN retry_time INT DEFAULT 60;
