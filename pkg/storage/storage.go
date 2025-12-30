package storage

import (
    "fmt"
	"os"
	"time"
)

type Storage struct{}

// FileStats holds metadata about a file without reading its contents.
type FileStats struct {
	SizeBytes int64
	ModTime   time.Time
}

func (s *Storage) SaveFile(filePath string, content []byte) error {
    err := os.WriteFile(filePath, content, 0644)
    if err != nil {
        return fmt.Errorf("error saving file: %s", err)
    }

    return nil
}

func (s *Storage) ReadFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	return data, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

func (s *Storage) HasFile(fn string) bool {
	if fileExists(fn) {
		return true
	}
	return false
}

// GetFileStats returns metadata about a file using os.Stat (no I/O overhead).
func (s *Storage) GetFileStats(filePath string) (*FileStats, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting file stats: %s", err)
	}

	return &FileStats{
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
	}, nil
}

