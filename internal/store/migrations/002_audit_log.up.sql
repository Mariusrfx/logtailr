-- 002_audit_log.up.sql
-- audit_log: CRUD operations performed via the API (F8.4)
CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    ts          TIMESTAMPTZ NOT NULL DEFAULT now(),
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    request_id  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_audit_log_ts ON audit_log (ts DESC);
CREATE INDEX idx_audit_log_resource ON audit_log (resource);
