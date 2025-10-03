package main

import (
	"log"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/service"
)

func main() {
	cfg, err := config.LoadConfig("configs/tags.yaml")
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	collector, err := service.NewCollectorService(cfg)
	if err != nil {
		log.Fatalf("Ошибка создания коллектора: %v", err)
	}

	if err := collector.Start(); err != nil {
		log.Fatalf("Ошибка при работе коллектора: %v", err)
	}

	log.Println("Сервис остановлен")
}
