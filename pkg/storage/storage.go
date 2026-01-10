// Package storage provides simple file storage operations.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Storage provides file storage and retrieval operations.
type Storage struct{}

// FileStats holds metadata about a file without reading its contents.
type FileStats struct {
	SizeBytes int64
	ModTime   time.Time
}

// SaveFile writes content to a file path.
func (s *Storage) SaveFile(filePath string, content []byte) error {
	err := os.WriteFile(filePath, content, 0600)
	if err != nil {
		return fmt.Errorf("error saving file: %w", err)
	}

	return nil
}

// ReadFile reads content from a file path.
func (s *Storage) ReadFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return data, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

// HasFile checks if a file exists at the given path.
func (s *Storage) HasFile(fn string) bool {
	return fileExists(fn)
}

// GetFileStats returns metadata about a file using os.Stat (no I/O overhead).
func (s *Storage) GetFileStats(filePath string) (*FileStats, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting file stats: %w", err)
	}

	return &FileStats{
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
	}, nil
}
