CREATE TABLE IF NOT EXISTS trips (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp INTEGER NOT NULL,
  name      TEXT NOT NULL,
  source    TEXT NOT NULL,
  UNIQUE(name),
  UNIQUE(timestamp)
);
CREATE INDEX IF NOT EXISTS trips_timestamp_idx ON trips(timestamp);

CREATE TABLE IF NOT EXISTS coords (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp INTEGER NOT NULL,
  message   TEXT NOT NULL DEFAULT '',
  longitude REAL NOT NULL,
  latitude  REAL NOT NULL,
  source    TEXT NOT NULL,
  UNIQUE(timestamp, longitude, latitude)
);
CREATE INDEX IF NOT EXISTS coords_timestamp_idx ON coords(timestamp);
CREATE INDEX IF NOT EXISTS coords_message_idx ON coords(message) WHERE message != '';

CREATE TABLE IF NOT EXISTS ingested_files (
  path        TEXT PRIMARY KEY,
  size_bytes  INTEGER NOT NULL,
  mod_time    INTEGER NOT NULL,
  ingested_at INTEGER NOT NULL
);
