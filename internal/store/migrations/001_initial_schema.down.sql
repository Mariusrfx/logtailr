-- 001_initial_schema.down.sql
-- Rollback: drop all tables and the trigger function

DROP TRIGGER IF EXISTS trg_bookmarks_updated_at ON bookmarks;
DROP TRIGGER IF EXISTS trg_saved_searches_updated_at ON saved_searches;
DROP TRIGGER IF EXISTS trg_settings_updated_at ON settings;
DROP TRIGGER IF EXISTS trg_alert_rules_updated_at ON alert_rules;
DROP TRIGGER IF EXISTS trg_outputs_updated_at ON outputs;
DROP TRIGGER IF EXISTS trg_sources_updated_at ON sources;

DROP FUNCTION IF EXISTS update_updated_at();

DROP TABLE IF EXISTS bookmarks;
DROP TABLE IF EXISTS saved_searches;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS alert_events;
DROP TABLE IF EXISTS alert_rules;
DROP TABLE IF EXISTS outputs;
DROP TABLE IF EXISTS sources;
