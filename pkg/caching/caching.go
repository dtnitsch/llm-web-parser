package caching

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cache provides a simple file-based cache with a TTL.
type Cache struct {
	path string
	ttl  time.Duration
}

// NewCache creates a new Cache instance.
// The cache path will be created if it doesn't exist.
func NewCache(path string, ttl time.Duration) (*Cache, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	return &Cache{
		path: path,
		ttl:  ttl,
	}, nil
}

// key generates a SHA256 hash of the URL to use as a filename.
func (c *Cache) key(url string) string {
	hash := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", hash)
}

// Get retrieves an item from the cache.
// It returns the data and true if the item is found and not expired.
// Otherwise, it returns nil and false.
func (c *Cache) Get(url string) ([]byte, bool) {
	filePath := filepath.Join(c.path, c.key(url))

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil, false // Cache miss
	}

	// Check if expired
	if time.Since(info.ModTime()) > c.ttl {
		return nil, false // Cache miss (expired)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false // Cache miss (read error)
	}

	return data, true // Cache hit
}

// Set adds an item to the cache.
func (c *Cache) Set(url string, data []byte) error {
	filePath := filepath.Join(c.path, c.key(url))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write to cache: %w", err)
	}
	return nil
}
