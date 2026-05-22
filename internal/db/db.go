package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

type FileChanges struct {
	Created  []string `json:"created"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

type RunRecord struct {
	ID        string      `json:"id"`
	Command   string      `json:"command"`
	Dir       string      `json:"dir"`
	Driver    string      `json:"driver"`
	StartedAt time.Time   `json:"started_at"`
	EndedAt   time.Time   `json:"ended_at"`
	ExitCode  int         `json:"exit_code"`
	Stdout    string      `json:"stdout"`
	Stderr    string      `json:"stderr"`
	Changes   FileChanges `json:"changes"`
}

// InitDB initializes the SQLite database at the specified path (defaults to ~/.glasswall/runs.db).
func InitDB(dbPath string) (*DB, error) {
	if dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}
		appDir := filepath.Join(homeDir, ".glasswall")
		if err := os.MkdirAll(appDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config dir: %w", err)
		}
		dbPath = filepath.Join(appDir, "runs.db")
	} else {
		// Ensure parent directory of custom dbPath exists
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create database parent directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		command TEXT NOT NULL,
		dir TEXT NOT NULL,
		driver TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		ended_at DATETIME NOT NULL,
		exit_code INTEGER NOT NULL,
		stdout TEXT NOT NULL,
		stderr TEXT NOT NULL,
		file_changes TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at);
	`
	_, err := db.conn.Exec(query)
	return err
}

// SaveRun saves a run record to SQLite.
func (db *DB) SaveRun(record *RunRecord) error {
	changesJSON, err := json.Marshal(record.Changes)
	if err != nil {
		return fmt.Errorf("failed to marshal file changes: %w", err)
	}

	query := `
	INSERT INTO runs (id, command, dir, driver, started_at, ended_at, exit_code, stdout, stderr, file_changes)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = db.conn.Exec(
		query,
		record.ID,
		record.Command,
		record.Dir,
		record.Driver,
		record.StartedAt,
		record.EndedAt,
		record.ExitCode,
		record.Stdout,
		record.Stderr,
		string(changesJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to insert run record: %w", err)
	}
	return nil
}

// GetRuns fetches all historical runs.
func (db *DB) GetRuns() ([]*RunRecord, error) {
	query := `
	SELECT id, command, dir, driver, started_at, ended_at, exit_code, stdout, stderr, file_changes
	FROM runs
	ORDER BY started_at DESC
	`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*RunRecord
	for rows.Next() {
		var record RunRecord
		var changesStr string
		var startedAtStr, endedAtStr string

		err := rows.Scan(
			&record.ID,
			&record.Command,
			&record.Dir,
			&record.Driver,
			&startedAtStr,
			&endedAtStr,
			&record.ExitCode,
			&record.Stdout,
			&record.Stderr,
			&changesStr,
		)
		if err != nil {
			return nil, err
		}

		record.StartedAt, _ = time.Parse(time.RFC3339, startedAtStr)
		if record.StartedAt.IsZero() {
			// fallback to standard SQLite time format parsed by library
			record.StartedAt, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", startedAtStr)
			if record.StartedAt.IsZero() {
				record.StartedAt, _ = time.Parse("2006-01-02 15:04:05.999999999", startedAtStr)
			}
		}

		record.EndedAt, _ = time.Parse(time.RFC3339, endedAtStr)
		if record.EndedAt.IsZero() {
			record.EndedAt, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", endedAtStr)
			if record.EndedAt.IsZero() {
				record.EndedAt, _ = time.Parse("2006-01-02 15:04:05.999999999", endedAtStr)
			}
		}

		if err := json.Unmarshal([]byte(changesStr), &record.Changes); err != nil {
			return nil, err
		}

		records = append(records, &record)
	}
	return records, nil
}

// GetRunByID fetches a single run by ID.
func (db *DB) GetRunByID(id string) (*RunRecord, error) {
	query := `
	SELECT id, command, dir, driver, started_at, ended_at, exit_code, stdout, stderr, file_changes
	FROM runs
	WHERE id = ?
	`
	row := db.conn.QueryRow(query, id)

	var record RunRecord
	var changesStr string
	var startedAtStr, endedAtStr string

	err := row.Scan(
		&record.ID,
		&record.Command,
		&record.Dir,
		&record.Driver,
		&startedAtStr,
		&endedAtStr,
		&record.ExitCode,
		&record.Stdout,
		&record.Stderr,
		&changesStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	record.StartedAt, _ = time.Parse(time.RFC3339, startedAtStr)
	if record.StartedAt.IsZero() {
		record.StartedAt, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", startedAtStr)
		if record.StartedAt.IsZero() {
			record.StartedAt, _ = time.Parse("2006-01-02 15:04:05.999999999", startedAtStr)
		}
	}

	record.EndedAt, _ = time.Parse(time.RFC3339, endedAtStr)
	if record.EndedAt.IsZero() {
		record.EndedAt, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", endedAtStr)
		if record.EndedAt.IsZero() {
			record.EndedAt, _ = time.Parse("2006-01-02 15:04:05.999999999", endedAtStr)
		}
	}

	if err := json.Unmarshal([]byte(changesStr), &record.Changes); err != nil {
		return nil, err
	}

	return &record, nil
}

