package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// PLCConfig представляет конфигурацию одного ПЛК
type PLCConfig struct {
	Host string `yaml:"host"`
	Slot int    `yaml:"slot"`
}

// TagConfig представляет конфигурацию тега
type TagConfig struct {
	PLC         string  `yaml:"plc"`                   // Имя ПЛК из секции plcs
	Type        string  `yaml:"type"`                  // Тип данных
	Description string  `yaml:"description"`           // Описание
	Unit        string  `yaml:"unit,omitempty"`        // Единица измерения
	ScaleFactor float64 `yaml:"scale_factor,omitempty"`// Коэффициент масштабирования
}

// DatabaseConfig представляет конфигурацию БД
type DatabaseConfig struct {
	Type      string `yaml:"type"`
	Database  string `yaml:"database"` // Путь к БД
}

// PollingConfig представляет конфигурацию опроса
type PollingConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

// Config представляет полную конфигурацию
type Config struct {
	PLCs     map[string]PLCConfig `yaml:"plcs"`     // Map ПЛК: имя -> конфиг
	Tags     map[string]TagConfig `yaml:"tags"`     // Map тегов: имя -> конфиг
	Database DatabaseConfig       `yaml:"database"`
	Polling  PollingConfig        `yaml:"polling"`
}

// LoadConfig загружает конфигурацию из YAML файла
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

// GetPLCConfig возвращает конфигурацию ПЛК для тега
func (c *Config) GetPLCConfig(tagName string) (*PLCConfig, error) {
	tagConfig, exists := c.Tags[tagName]
	if !exists {
		return nil, fmt.Errorf("тег %s не найден в конфигурации", tagName)
	}

	plcConfig, exists := c.PLCs[tagConfig.PLC]
	if !exists {
		return nil, fmt.Errorf("ПЛК %s для тега %s не найден", tagConfig.PLC, tagName)
	}

	return &plcConfig, nil
}

// GetTagsByPLC возвращает все теги для указанного ПЛК
func (c *Config) GetTagsByPLC(plcName string) map[string]TagConfig {
	result := make(map[string]TagConfig)
	for tagName, tagConfig := range c.Tags {
		if tagConfig.PLC == plcName {
			result[tagName] = tagConfig
		}
	}
	return result
}

// Validate проверяет корректность конфигурации
func (c *Config) Validate() error {
	// Проверяем что есть ПЛК
	if len(c.PLCs) == 0 {
		return fmt.Errorf("не указаны ПЛК в конфигурации")
	}

	// Проверяем что есть теги
	if len(c.Tags) == 0 {
		return fmt.Errorf("не указаны теги в конфигурации")
	}

	// Проверяем что все теги ссылаются на существующие ПЛК
	for tagName, tagConfig := range c.Tags {
		if _, exists := c.PLCs[tagConfig.PLC]; !exists {
			return fmt.Errorf("тег %s ссылается на несуществующий ПЛК %s", tagName, tagConfig.PLC)
		}
	}

	return nil
}
