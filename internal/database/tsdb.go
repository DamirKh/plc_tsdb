package database // Важно: имя пакета = имени папки

import (
	"fmt"
	"log"
	"plc_tsdb/internal/config"
	"time"
)

type TSDBClient interface {
	Write(data map[string]interface{}, timestamp time.Time) error
	Close() error
}

func NewTSDBClient(cfg *config.DatabaseConfig) (TSDBClient, error) {
	switch cfg.Type {
	case "sqlite":
		return NewSQLiteClient(cfg)
	case "mock":
		return &MockTSDBClient{}, nil
	default:
		return nil, fmt.Errorf("неподдерживаемый тип БД: %s", cfg.Type)
	}
}

// Mock клиент для тестов
type MockTSDBClient struct{}

func (m *MockTSDBClient) Write(data map[string]interface{}, timestamp time.Time) error {
	log.Printf("[MOCK] Запись данных: %+v", data)
	return nil
}

func (m *MockTSDBClient) Close() error {
	return nil
}
