package plc

import (
	"fmt"
	"log"

	// "time"

	"plc_tsdb/internal/config"

	"github.com/danomagnum/gologix"
)

// PLCClient представляет клиент для одного ПЛК
type PLCClient struct {
	name        string
	client      *gologix.Client
	config      *config.PLCConfig
	isConnected bool
}

// PLCManager управляет несколькими клиентами ПЛК
type PLCManager struct {
	clients map[string]*PLCClient
	config  *config.Config
}

// NewPLCManager создает менеджер для работы с несколькими ПЛК
func NewPLCManager(cfg *config.Config) *PLCManager {
	manager := &PLCManager{
		clients: make(map[string]*PLCClient),
		config:  cfg,
	}

	// Создаем клиенты для каждого ПЛК
	for plcName, plcConfig := range cfg.PLCs {
		manager.clients[plcName] = &PLCClient{
			name:   plcName,
			config: &plcConfig,
			client: gologix.NewClient(plcConfig.Host),
		}
	}

	return manager
}

// Connect подключается ко всем ПЛК
func (m *PLCManager) Connect() error {
	var errors []string

	for plcName, client := range m.clients {
		if err := client.Connect(); err != nil {
			errors = append(errors, fmt.Sprintf("ПЛК %s: %v", plcName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("ошибки подключения: %v", errors)
	}

	return nil
}

// Disconnect отключается от всех ПЛК
func (m *PLCManager) Disconnect() {
	for _, client := range m.clients {
		client.Disconnect()
	}
}

// ReadAllTags читает все теги со всех ПЛК
func (m *PLCManager) ReadAllTags() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	var errors []string

	for plcName, client := range m.clients {
		if !client.isConnected {
			errors = append(errors, fmt.Sprintf("ПЛК %s не подключен", plcName))
			continue
		}

		// Получаем теги для этого ПЛК
		tagsForPLC := m.config.GetTagsByPLC(plcName)
		if len(tagsForPLC) == 0 {
			continue
		}

		// Читаем теги с ПЛК
		plcTags, err := client.readTags(tagsForPLC)
		if err != nil {
			errors = append(errors, fmt.Sprintf("ПЛК %s: %v", plcName, err))
			continue
		}

		// Добавляем теги в результат с префиксом ПЛК
		for tagName, value := range plcTags {
			fullTagName := fmt.Sprintf("%s/%s", plcName, tagName)
			result[fullTagName] = value
		}
	}

	if len(errors) > 0 {
		return result, fmt.Errorf("ошибки чтения: %v", errors)
	}

	return result, nil
}

// ReadTags читает конкретные теги
func (m *PLCManager) ReadTags(tagNames []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, tagName := range tagNames {
		tagConfig, exists := m.config.Tags[tagName]
		if !exists {
			log.Printf("Тег %s не найден в конфигурации", tagName)
			continue
		}

		plcClient, exists := m.clients[tagConfig.PLC]
		if !exists || !plcClient.isConnected {
			log.Printf("ПЛК %s для тега %s недоступен", tagConfig.PLC, tagName)
			continue
		}

		value, err := plcClient.readSingleTag(tagName, tagConfig)
		if err != nil {
			log.Printf("Ошибка чтения тега %s: %v", tagName, err)
			continue
		}

		fullTagName := fmt.Sprintf("%s/%s", tagConfig.PLC, tagName)
		result[fullTagName] = value
	}

	return result, nil
}

// Connect подключает один ПЛК
func (c *PLCClient) Connect() error {
	// Настройка пути при необходимости
	// if c.config.Slot != 0 {
	// 	path, err := gologix.ParsePath(fmt.Sprintf("1,%d", c.config.Slot))
	// 	if err == nil {
	// 		c.client.Path = path
	// 	}
	// }

	err := c.client.Connect()
	if err != nil {
		return fmt.Errorf("ошибка подключения к ПЛК %s: %w", c.name, err)
	}

	c.isConnected = true
	log.Printf("Успешно подключен к ПЛК %s (%s)", c.name, c.config.Host)
	return nil
}

// Disconnect отключает ПЛК
func (c *PLCClient) Disconnect() {
	if c.isConnected {
		c.client.Disconnect()
		c.isConnected = false
		log.Printf("Отключен от ПЛК %s", c.name)
	}
}

// readTags читает теги для одного ПЛК
func (c *PLCClient) readTags(tags map[string]config.TagConfig) (map[string]interface{}, error) {
	tagMap := make(map[string]interface{})

	for tagName, tagConfig := range tags {
		switch tagConfig.Type {
		case "float32":
			tagMap[tagName] = float32(0)
		case "int32":
			tagMap[tagName] = int32(0)
		case "bool":
			tagMap[tagName] = false
		// ... другие типы
		default:
			log.Printf("Неподдерживаемый тип тега %s: %s", tagName, tagConfig.Type)
		}
	}

	if err := c.client.ReadMulti(tagMap); err != nil {
		return nil, fmt.Errorf("ошибка чтения тегов: %w", err)
	}

	// Применяем масштабирование
	c.applyScaleFactors(tagMap, tags)

	return tagMap, nil
}

// readSingleTag читает один тег
func (c *PLCClient) readSingleTag(tagName string, tagConfig config.TagConfig) (interface{}, error) {
	var value interface{}

	switch tagConfig.Type {
	case "float32":
		value = float32(0)
	case "int32":
		value = int32(0)
	case "bool":
		value = false
	default:
		return nil, fmt.Errorf("неподдерживаемый тип: %s", tagConfig.Type)
	}

	err := c.client.Read(tagName, value)
	if err != nil {
		return nil, err
	}

	// Применяем масштабирование
	if tagConfig.ScaleFactor != 0 && tagConfig.ScaleFactor != 1.0 {
		switch v := value.(type) {
		case float32:
			value = v * float32(tagConfig.ScaleFactor)
		case int32:
			value = float32(v) * float32(tagConfig.ScaleFactor)
		}
	}

	return value, nil
}

// applyScaleFactors применяет коэффициенты масштабирования
func (c *PLCClient) applyScaleFactors(tags map[string]interface{}, tagConfigs map[string]config.TagConfig) {
	for tagName, value := range tags {
		if tagConfig, exists := tagConfigs[tagName]; exists {
			if tagConfig.ScaleFactor != 0 && tagConfig.ScaleFactor != 1.0 {
				switch v := value.(type) {
				case float32:
					tags[tagName] = v * float32(tagConfig.ScaleFactor)
				case int32:
					tags[tagName] = float32(v) * float32(tagConfig.ScaleFactor)
				}
			}
		}
	}
}

// GetConnectionStatus возвращает статус подключения ПЛК
func (m *PLCManager) GetConnectionStatus() map[string]bool {
	status := make(map[string]bool)
	for plcName, client := range m.clients {
		status[plcName] = client.isConnected
	}
	return status
}
