package config // Важно: имя пакета = имени папки

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type TagConfig struct {
	Type        string  `yaml:"type"`
	Description string  `yaml:"description"`
	ScaleFactor float64 `yaml:"scale_factor,omitempty"`
	Length      int     `yaml:"length,omitempty"`
}

type PLCConfig struct {
	Host string `yaml:"host"`
	Slot int    `yaml:"slot"`
}

type DatabaseConfig struct {
	Type          string        `yaml:"type"`
	URL           string        `yaml:"url"`
	Database      string        `yaml:"database"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

type PollingConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

type Config struct {
	PLC      PLCConfig            `yaml:"plc_connection"`
	Tags     map[string]TagConfig `yaml:"tags"`
	Database DatabaseConfig       `yaml:"database"`
	Polling  PollingConfig        `yaml:"polling"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения конфига: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("ошибка парсинга YAML: %w", err)
	}

	return &config, nil
}
