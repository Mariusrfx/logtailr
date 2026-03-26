-- 001_initial_schema.up.sql
-- Logtailr: initial PostgreSQL schema

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- sources: log source configurations (replaces config.yaml → sources[])
CREATE TABLE sources (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL UNIQUE,
    type           TEXT NOT NULL CHECK (type IN ('file','docker','journalctl','stdin','kubernetes')),
    path           TEXT NOT NULL DEFAULT '',
    container      TEXT NOT NULL DEFAULT '',
    unit           TEXT NOT NULL DEFAULT '',
    priority       TEXT NOT NULL DEFAULT '',
    output_format  TEXT NOT NULL DEFAULT '',
    namespace      TEXT NOT NULL DEFAULT '',
    pod            TEXT NOT NULL DEFAULT '',
    label_selector TEXT NOT NULL DEFAULT '',
    kubeconfig     TEXT NOT NULL DEFAULT '',
    follow         BOOLEAN NOT NULL DEFAULT true,
    parser         TEXT NOT NULL DEFAULT '' CHECK (parser IN ('','json','logfmt','text')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sources_type ON sources (type);

-- outputs: output destinations (replaces config.yaml → outputs{})
CREATE TABLE outputs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    type       TEXT NOT NULL CHECK (type IN ('opensearch','webhook','file')),
    config     JSONB NOT NULL DEFAULT '{}',
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- alert_rules: alert rule definitions (replaces config.yaml → alerts.rules[])
CREATE TABLE alert_rules (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      TEXT NOT NULL UNIQUE,
    type      TEXT NOT NULL CHECK (type IN ('pattern','level','error_rate','health_change')),
    severity  TEXT NOT NULL CHECK (severity IN ('warning','critical')),
    pattern   TEXT NOT NULL DEFAULT '',
    level     TEXT NOT NULL DEFAULT '',
    source    TEXT NOT NULL DEFAULT '',
    threshold INTEGER NOT NULL DEFAULT 0,
    window    TEXT NOT NULL DEFAULT '',
    cooldown  TEXT NOT NULL DEFAULT '',
    enabled   BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- alert_events: fired alert history (replaces in-memory Engine.recent)
CREATE TABLE alert_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_name       TEXT NOT NULL,
    severity        TEXT NOT NULL CHECK (severity IN ('warning','critical')),
    message         TEXT NOT NULL DEFAULT '',
    source          TEXT NOT NULL DEFAULT '',
    count           INTEGER NOT NULL DEFAULT 1,
    fired_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at TIMESTAMPTZ
);

CREATE INDEX idx_alert_events_fired_at ON alert_events (fired_at DESC);
CREATE INDEX idx_alert_events_severity ON alert_events (severity);
CREATE INDEX idx_alert_events_rule_name ON alert_events (rule_name);

-- settings: global key-value settings (replaces config.yaml → global{})
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL DEFAULT 'null',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- saved_searches: user-saved filter combinations (new feature)
CREATE TABLE saved_searches (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    filters    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- bookmarks: file reading positions (replaces ~/.logtailr/bookmarks.json)
CREATE TABLE bookmarks (
    name     TEXT PRIMARY KEY,
    file     TEXT NOT NULL DEFAULT '',
    "offset" BIGINT NOT NULL DEFAULT 0,
    inode    BIGINT NOT NULL DEFAULT 0,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- trigger function to auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_sources_updated_at BEFORE UPDATE ON sources FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_outputs_updated_at BEFORE UPDATE ON outputs FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_alert_rules_updated_at BEFORE UPDATE ON alert_rules FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_settings_updated_at BEFORE UPDATE ON settings FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_saved_searches_updated_at BEFORE UPDATE ON saved_searches FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_bookmarks_updated_at BEFORE UPDATE ON bookmarks FOR EACH ROW EXECUTE FUNCTION update_updated_at();
