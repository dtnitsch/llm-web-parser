// Package models defines data structures for configuration and parsing.
package models

import (
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

// Config represents the application configuration loaded from YAML.
type Config struct {
	URLs      []string                     `yaml:"urls"`
	MapReduce MapReduceConfig              `yaml:"mapreduce"`
	Analytics map[string]AnalyticsConfig   `yaml:"analytics"`
	WorkerCount int `yaml:"worker_count"`
}

// MapReduceConfig contains configuration for map-reduce operations.
type MapReduceConfig struct {
	ReduceKeys []string `yaml:"reduce_keys"`
}

// AnalyticsConfig contains configuration for analytics operations.
type AnalyticsConfig struct {
	KeywordThreshold int `yaml:"keyword_threshold"`
}

// LoadConfig reads and parses a YAML configuration file.
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	return &config, nil
}
