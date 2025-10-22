package plc

import (
	"fmt"
	"sync"

	// "time"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/logging"

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

	// Создаём общий gologix.Logger, связанный с нашим slog
	var goLogger gologix.LoggerInterface
	if l, ok := gologix.NewLogger().(*gologix.Logger); ok {
		l.SetLogger(logging.Logger) // logging.Logger — твой *slog.Logger
		goLogger = l
	}

	// Создаем клиентов для каждого ПЛК
	for plcName, plcConfig := range cfg.PLCs {
		client := gologix.NewClient(plcConfig.Host)

		// Назначаем логгер клиенту, если поддерживается
		if goLogger != nil {
			client.Logger = goLogger
		}

		manager.clients[plcName] = &PLCClient{
			name:   plcName,
			config: &plcConfig,
			client: client,
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

// ReadAllTags читает все теги со всех ПЛК параллельно
func (m *PLCManager) ReadAllTags() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	var errors []string

	var mu sync.Mutex
	var wg sync.WaitGroup

	for plcName, client := range m.clients {
		wg.Add(1)

		go func(plcName string, client *PLCClient) {
			defer wg.Done()

			if !client.isConnected {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ПЛК %s не подключен", plcName))
				mu.Unlock()
				return
			}

			// Получаем теги для этого ПЛК
			tagsForPLC := m.config.GetTagsByPLC(plcName)
			if len(tagsForPLC) == 0 {
				return
			}

			// Читаем теги
			plcTags, err := client.readTags(tagsForPLC)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ПЛК %s: %v", plcName, err))
				mu.Unlock()
				return
			}

			// Добавляем теги в общий результат
			mu.Lock()
			for tagName, value := range plcTags {
				fullTagName := fmt.Sprintf("%s/%s", plcName, tagName)
				result[fullTagName] = value
			}
			mu.Unlock()
		}(plcName, client)
	}

	wg.Wait()

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
			logging.Error("Тег не найден в конфигурации", "tagName", tagName)
			continue
		}

		plcClient, exists := m.clients[tagConfig.PLC]
		if !exists || !plcClient.isConnected {
			logging.Warn("ПЛК недоступен", "PLC", tagConfig.PLC)
			continue
		}

		value, err := plcClient.readSingleTag(tagName, tagConfig)
		if err != nil {
			logging.Error("Ошибка чтения тега", "tagName", err)
			continue
		}

		fullTagName := fmt.Sprintf("%s/%s", tagConfig.PLC, tagName)
		result[fullTagName] = value
	}

	return result, nil
}

// Connect подключает один ПЛК
func (c *PLCClient) Connect() error {
	err := c.client.Connect()
	if err != nil {
		return fmt.Errorf("ошибка подключения к ПЛК %s: %w", c.name, err)
	}

	c.isConnected = true
	logging.Info("Успешно подключен к ПЛК", "PLC", c.name, "IP", c.config.Host)
	return nil
}

// Disconnect отключает ПЛК
func (c *PLCClient) Disconnect() {
	if c.isConnected {
		c.client.Disconnect()
		c.isConnected = false
		logging.Info("Отключен от ПЛК", "PLC", c.name)
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
			logging.Error("Неподдерживаемый тип тега", "TagName", tagName, "Type", tagConfig.Type)
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
