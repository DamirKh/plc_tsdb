package service

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

type CollectorService struct {
	plcManager *plc.PLCManager
	dbClient   database.TSDBClient
	config     *config.Config
	stopChan   chan struct{}
}

func NewCollectorService(cfg *config.Config) (*CollectorService, error) {
	plcManager := plc.NewPLCManager(cfg)

	dbClient, err := database.NewTSDBClient(&cfg.Database)
	if err != nil {
		return nil, err
	}

	return &CollectorService{
		plcManager: plcManager,
		dbClient:   dbClient,
		config:     cfg,
		stopChan:   make(chan struct{}),
	}, nil
}

func (s *CollectorService) Start() error {
	if err := s.plcManager.Connect(); err != nil {
		return err
	}
	defer s.plcManager.Disconnect()

	log.Printf("Запуск сбора данных с интервалом %v", s.config.Polling.Interval)

	ticker := time.NewTicker(s.config.Polling.Interval)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			s.collectData()
		case <-sigChan:
			log.Println("Получен сигнал остановки")
			return nil
		case <-s.stopChan:
			log.Println("Остановка по команде")
			return nil
		}
	}
}

func (s *CollectorService) collectData() {
	tags, err := s.plcManager.ReadAllTags()
	if err != nil {
		log.Printf("Ошибка чтения тегов: %v", err)
	}

	timestamp := time.Now()
	if err := s.dbClient.Write(tags, timestamp); err != nil {
		log.Printf("Ошибка записи в TSDB: %v", err)
		return
	}

	log.Printf("Успешно записано %d тегов в %v", len(tags), timestamp)
}

func (s *CollectorService) Stop() {
	close(s.stopChan)
	s.dbClient.Close()
}
