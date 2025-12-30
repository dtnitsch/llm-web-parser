package models

import (
	"os"

	yaml "gopkg.in/yaml.v3"
)

type Config struct {
	URLs      []string                     `yaml:"urls"`
	MapReduce MapReduceConfig              `yaml:"mapreduce"`
	Analytics map[string]AnalyticsConfig   `yaml:"analytics"`
	WorkerCount int `yaml:"worker_count"`
}

type MapReduceConfig struct {
	ReduceKeys []string `yaml:"reduce_keys"`
}

type AnalyticsConfig struct {
	KeywordThreshold int `yaml:"keyword_threshold"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
