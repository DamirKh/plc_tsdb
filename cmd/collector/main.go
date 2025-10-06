package main

import (
	"log"
	"os"
	"path/filepath"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/service"
)

func main() {
	// Определяем путь к конфигу относительно исполняемого файла
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Ошибка определения пути: %v", err)
	}
	exeDir := filepath.Dir(exePath)

	// Пробуем разные пути к конфигу
	configPaths := []string{
		filepath.Join(exeDir, "configs", "tags.yaml"),
		filepath.Join(exeDir, "..", "configs", "tags.yaml"),
		"configs/tags.yaml",
		"../../configs/tags.yaml",
		"C:/pls_tsdb/configs/tags.yaml",
	}

	var cfg *config.Config
	for _, configPath := range configPaths {
		cfg, err = config.LoadConfig(configPath)
		if err == nil {
			log.Printf("Конфиг загружен: %s", configPath)
			break
		}
	}

	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создаем необходимые директории
	os.MkdirAll("logs", 0755)
	os.MkdirAll("data", 0755)

	collector, err := service.NewCollectorService(cfg)
	if err != nil {
		log.Fatalf("Ошибка создания коллектора: %v", err)
	}

	log.Println("Запуск PLC Data Collector сервиса...")
	if err := collector.Start(); err != nil {
		log.Fatalf("Ошибка при работе коллектора: %v", err)
	}

	log.Println("Сервис остановлен")
}
