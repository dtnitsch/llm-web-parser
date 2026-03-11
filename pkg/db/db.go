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

// Open opens or creates the SQLite database in the current working directory
func Open() (*DB, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	dbPath := filepath.Join(cwd, DefaultDBName)

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

	// Run migrations for existing databases
	if err := db.runMigrations(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
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

// runMigrations runs schema migrations for existing databases
func (db *DB) runMigrations() error {
	// Migration 1: Add meta_keywords column (2026-03-10)
	// Check if column exists by querying table info
	var hasMetaKeywords bool
	rows, err := db.Query("PRAGMA table_info(urls)")
	if err != nil {
		return fmt.Errorf("failed to check table schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}
		if name == "meta_keywords" {
			hasMetaKeywords = true
			break
		}
	}

	if !hasMetaKeywords {
		_, err = db.Exec("ALTER TABLE urls ADD COLUMN meta_keywords TEXT")
		if err != nil {
			return fmt.Errorf("failed to add meta_keywords column: %w", err)
		}
	}

	return nil
}
