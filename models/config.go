// Package models defines data structures for configuration and parsing.
package models

// FetchConfig holds runtime configuration for fetch operations.
// All values come from CLI flags, not external config files.
type FetchConfig struct {
	URLs        []string
	WorkerCount int
}
