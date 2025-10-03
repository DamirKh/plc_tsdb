package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/database"
	"plc_tsdb/internal/plc"
)

type DataCollector struct {
	plcClient  *plc.PLCClient
	tsdbClient database.TSDBClient
	config     *config.Config
	stopChan   chan struct{}
}

func NewDataCollector(cfg *config.Config) (*DataCollector, error) {
	plcClient := plc.NewPLCClient(cfg)

	tsdbClient, err := database.NewTSDBClient(&cfg.Database)
	if err != nil {
		return nil, err
	}

	return &DataCollector{
		plcClient:  plcClient,
		tsdbClient: tsdbClient,
		config:     cfg,
		stopChan:   make(chan struct{}),
	}, nil
}

func (d *DataCollector) Start() error {
	// Подключаемся к ПЛК
	if err := d.plcClient.Connect(); err != nil {
		return err
	}
	defer d.plcClient.Disconnect()

	log.Printf("Запуск сбора данных с интервалом %v", d.config.Polling.Interval)

	ticker := time.NewTicker(d.config.Polling.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.collectData()
		case <-d.stopChan:
			log.Println("Остановка сбора данных")
			return nil
		}
	}
}

func (d *DataCollector) collectData() {
	tags, err := d.plcClient.ReadTags()
	if err != nil {
		log.Printf("Ошибка чтения тегов: %v", err)
		return
	}

	timestamp := time.Now()
	if err := d.tsdbClient.Write(tags, timestamp); err != nil {
		log.Printf("Ошибка записи в TSDB: %v", err)
		return
	}

	log.Printf("Успешно записано %d тегов в %v", len(tags), timestamp)
}

func (d *DataCollector) Stop() {
	close(d.stopChan)
	d.tsdbClient.Close()
}

func main() {
	// Загрузка конфигурации
	cfg, err := config.LoadConfig("configs/tags.yaml")
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создание коллектора
	collector, err := NewDataCollector(cfg)
	if err != nil {
		log.Fatalf("Ошибка создания коллектора: %v", err)
	}

	// Обработка сигналов остановки
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Получен сигнал остановки")
		collector.Stop()
	}()

	// Запуск сбора данных
	if err := collector.Start(); err != nil {
		log.Fatalf("Ошибка при работе коллектора: %v", err)
	}

	log.Println("Сервис остановлен")
}
