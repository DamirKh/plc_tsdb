package main

import (
	"flag"
	"os"

	// "path/filepath"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/logging"
	"plc_tsdb/internal/service"

	"github.com/danomagnum/gologix"
)

func main() {
	var configPath, logDir string
	flag.StringVar(&configPath, "config", "", "Путь к файлу конфигурации (tags.yaml)")
	flag.StringVar(&logDir, "logdir", "", "Каталог для логов (по умолчанию stdout/stderr)")
	flag.Parse()

	// Инициализация логгера
	if err := logging.Init(logDir); err != nil {
		logging.Error("Ошибка инициализации логгера", "error", err)
		os.Exit(1)
	}

	logging.Info("PLC_TSDB стартует...")

	// Загружаем конфиг
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logging.Error("Ошибка загрузки конфигурации", "error", err)
		os.Exit(1)
	}
	logging.Info("Конфигурация успешно загружена", "path", configPath)

	// Передаём наш логгер в gologix
	gologix.SetLogger(logging.Logger)

	// Создаём необходимые директории
	os.MkdirAll("data", 0755)

	collector, err := service.NewCollectorService(cfg)
	if err != nil {
		logging.Error("Ошибка создания коллектора", "error", err)
		os.Exit(1)
	}

	logging.Info("Запуск PLC Data Collector сервиса...")
	if err := collector.Start(); err != nil {
		logging.Error("Ошибка при работе коллектора", "error", err)
	}

	logging.Info("Сервис остановлен")
}
