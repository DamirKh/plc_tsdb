package service

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/database"
	"plc_tsdb/internal/logging"
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

	logging.Info("Запуск сбора данных,", "интервал", s.config.Polling.Interval)

	ticker := time.NewTicker(s.config.Polling.Interval)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			s.collectData()
		case <-sigChan:
			logging.Info("Получен сигнал остановки")
			return nil
		case <-s.stopChan:
			logging.Info("Остановка по команде")
			return nil
		}
	}
}

func (s *CollectorService) collectData() {
	tags, err := s.plcManager.ReadAllTags()
	if err != nil {
		logging.Error("Ошибка чтения тегов:", "Error", err)
	}

	timestamp := time.Now()
	if err := s.dbClient.Write(tags, timestamp); err != nil {
		logging.Error("Ошибка записи в TSDB^", "Error", err)
		return
	}

	logging.Debug("Записано успешно в TSDB:", "кол-во тегов", len(tags), "время", timestamp)
}

func (s *CollectorService) Stop() {
	close(s.stopChan)
	s.dbClient.Close()
}
