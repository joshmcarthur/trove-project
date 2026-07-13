package journal

const recordsSchemaDDL = `
CREATE TABLE IF NOT EXISTS record_heads (
  record_ref   TEXT PRIMARY KEY,
  version      INTEGER NOT NULL,
  type         TEXT,
  source       TEXT NOT NULL,
  body         TEXT NOT NULL,
  content_ref  TEXT,
  completeness TEXT NOT NULL,
  updated_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_record_heads_completeness ON record_heads(completeness);
CREATE INDEX IF NOT EXISTS idx_record_heads_source ON record_heads(source);

CREATE TABLE IF NOT EXISTS record_events (
  record_ref TEXT NOT NULL,
  version    INTEGER NOT NULL,
  event_id   TEXT NOT NULL,
  PRIMARY KEY (record_ref, version)
);
CREATE INDEX IF NOT EXISTS idx_record_events_event_id ON record_events(event_id);

CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
  record_ref UNINDEXED,
  type,
  source,
  body,
  tokenize = 'porter'
);
`
