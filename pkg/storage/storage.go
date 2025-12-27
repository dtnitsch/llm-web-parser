package storage

import (
    "fmt"
	"os"
)

type Storage struct{}

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

