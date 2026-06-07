-- Multi-sink delivery (webhook, redis, rabbitmq, kafka) — Sequin-style sink_type + sink_config.
ALTER TABLE lc_webhooks
    ADD COLUMN IF NOT EXISTS sink_type   TEXT NOT NULL DEFAULT 'webhook',
    ADD COLUMN IF NOT EXISTS sink_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE lc_webhooks
    ALTER COLUMN target_url DROP NOT NULL;

UPDATE lc_webhooks SET sink_type = 'webhook' WHERE sink_type IS NULL OR sink_type = '';
