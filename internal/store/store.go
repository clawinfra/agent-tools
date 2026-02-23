// Package store provides SQLite persistence for agent-tools.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// DB wraps a sql.DB with agent-tools-specific methods.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	wrapped := &DB{db}
	if err := wrapped.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return wrapped, nil
}

// migrate runs all schema migrations idempotently.
func (db *DB) migrate() error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS providers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    endpoint    TEXT NOT NULL,
    pubkey      TEXT NOT NULL,
    stake_claw  TEXT NOT NULL DEFAULT '0',
    reputation  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    last_seen   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tools (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    version     TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    schema_json TEXT NOT NULL,
    pricing     TEXT NOT NULL,
    provider_id TEXT NOT NULL REFERENCES providers(id),
    endpoint    TEXT NOT NULL,
    timeout_ms  INTEGER NOT NULL DEFAULT 30000,
    tags        TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    is_active   INTEGER NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX IF NOT EXISTS tools_name_version_provider 
    ON tools(name, version, provider_id) WHERE is_active = 1;

CREATE VIRTUAL TABLE IF NOT EXISTS tools_fts USING fts5(
    name, description, tags,
    content='tools',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS tools_fts_insert AFTER INSERT ON tools BEGIN
    INSERT INTO tools_fts(rowid, name, description, tags)
    VALUES (new.rowid, new.name, new.description, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS tools_fts_update AFTER UPDATE ON tools BEGIN
    INSERT INTO tools_fts(tools_fts, rowid, name, description, tags)
    VALUES ('delete', old.rowid, old.name, old.description, old.tags);
    INSERT INTO tools_fts(rowid, name, description, tags)
    VALUES (new.rowid, new.name, new.description, new.tags);
END;

CREATE TABLE IF NOT EXISTS invocations (
    id              TEXT PRIMARY KEY,
    tool_id         TEXT NOT NULL REFERENCES tools(id),
    consumer_id     TEXT NOT NULL,
    input_hash      TEXT NOT NULL,
    output_hash     TEXT,
    receipt_sig     TEXT,
    status          TEXT NOT NULL DEFAULT 'pending',
    cost_claw       TEXT,
    escrow_id       TEXT,
    started_at      INTEGER NOT NULL,
    completed_at    INTEGER,
    error           TEXT
);
`
