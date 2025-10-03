package plc

import (
	"fmt"
	"log"

	"plc_tsdb/internal/config"

	"github.com/danomagnum/gologix"
)

type PLCClient struct {
	client      *gologix.Client
	config      *config.Config
	isConnected bool
}

func NewPLCClient(cfg *config.Config) *PLCClient {
	// В новых версиях gologix путь задается через опции
	client := gologix.NewClient(cfg.PLC.Host)

	return &PLCClient{
		client: client,
		config: cfg,
	}
}

func (p *PLCClient) Connect() error {
	// В новых версиях Connect() может принимать параметры
	err := p.client.Connect()
	if err != nil {
		return fmt.Errorf("ошибка подключения к ПЛК: %w", err)
	}
	p.isConnected = true
	log.Printf("Успешно подключен к ПЛК %s", p.config.PLC.Host)
	return nil
}

func (p *PLCClient) Disconnect() {
	if p.isConnected {
		p.client.Disconnect()
		p.isConnected = false
		log.Println("Отключен от ПЛК")
	}
}

func (p *PLCClient) ReadTags() (map[string]interface{}, error) {
	if !p.isConnected {
		return nil, fmt.Errorf("клиент не подключен к ПЛК")
	}

	tags := make(map[string]interface{})

	for tagName, tagConfig := range p.config.Tags {
		// Создаем переменные ПРАВИЛЬНЫХ типов
		switch tagConfig.Type {
		case "float32", "float64":
			tags[tagName] = float32(0)
		case "int16":
			tags[tagName] = int16(0)
		case "int32":
			tags[tagName] = int32(0)
		case "int64":
			tags[tagName] = int64(0) // !
		case "int":
			tags[tagName] = int(0)
		case "bool":
			tags[tagName] = false
		case "uint16":
			tags[tagName] = uint16(0)
		case "uint32":
			tags[tagName] = uint32(0)
		default:
			log.Printf("Неподдерживаемый тип тега %s: %s", tagName, tagConfig.Type)
			continue
		}
	}

	// Чтение тегов из ПЛК
	if err := p.client.ReadMulti(tags); err != nil {
		return nil, fmt.Errorf("ошибка чтения тегов: %w", err)
	}

	// Применяем масштабирующие коэффициенты
	p.applyScaleFactors(tags)

	// Логируем успешное чтение
	if len(tags) > 0 {
		log.Printf("Успешно прочитано %d тегов из ПЛК", len(tags))
	}

	return tags, nil
}

func (p *PLCClient) applyScaleFactors(tags map[string]interface{}) {
	for tagName, value := range tags {
		if tagConfig, exists := p.config.Tags[tagName]; exists {
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

func (p *PLCClient) IsConnected() bool {
	return p.isConnected
}
