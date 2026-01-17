package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const DefaultDBName = "llm-web-parser.db"

type DB struct {
	*sql.DB
	path string
}

// openDB opens a SQLite database at the given path
func openDB(dbPath string) (*sql.DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = sqlDB.Close() // Close error less important than PRAGMA error
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return sqlDB, nil
}

// Open opens or creates the SQLite database next to the binary
func Open() (*DB, error) {
	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	dbPath := filepath.Join(execDir, DefaultDBName)

	sqlDB, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{
		DB:   sqlDB,
		path: dbPath,
	}

	// Auto-initialize schema if it doesn't exist
	if err := db.ensureSchemaExists(); err != nil {
		_ = db.Close() // Close error less important than schema error
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// ensureSchemaExists checks if the schema exists and initializes it if not
func (db *DB) ensureSchemaExists() error {
	// Check if the urls table exists (simple schema check)
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='urls'").Scan(&tableName)

	if err == sql.ErrNoRows {
		// Schema doesn't exist, initialize it
		return db.InitSchema()
	}

	if err != nil {
		return fmt.Errorf("failed to check schema: %w", err)
	}

	// Schema exists, all good
	return nil
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// InitSchema initializes the database schema
func (db *DB) InitSchema() error {
	_, err := db.Exec(schema)
	return err
}
