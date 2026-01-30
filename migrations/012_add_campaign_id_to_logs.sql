-- Migración 012: Agregar campaign_id a call logs
-- Permite filtrar reportes de llamadas por campaña

ALTER TABLE apicall_call_log ADD COLUMN campaign_id INT NULL DEFAULT NULL;
ALTER TABLE apicall_call_log ADD INDEX idx_campaign (campaign_id);
ALTER TABLE apicall_call_log ADD CONSTRAINT fk_call_log_campaign FOREIGN KEY (campaign_id) REFERENCES apicall_campaigns(id) ON DELETE SET NULL;
