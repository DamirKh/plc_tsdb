package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"plc_tsdb/internal/config"
	"plc_tsdb/internal/service"
)

var (
	exeDir     string
	configPath string
	logDir     string
)

// initLogger настраивает логирование
func initLogger(logDir string, useFile bool) *os.File {
	var file *os.File
	var err error

	if useFile {
		if logDir == "" {
			logDir = "logs"
		}
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Fatalf("Ошибка создания директории логов: %v", err)
		}

		logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
		file, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Ошибка открытия файла логов: %v", err)
		}

		// stdout + файл
		multiWriter := io.MultiWriter(os.Stdout, file)
		log.SetOutput(multiWriter)
		log.Printf("=== Запуск приложения ===")
		log.Printf("Логи записываются в: %s", logFile)

	} else {
		// Без файлов — только stdout / stderr
		log.SetOutput(os.Stdout)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	return file
}

// logError пишет в stderr, но не прерывает выполнение
func logError(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", v...)
}

func main() {
	flag.StringVar(&configPath, "config", "", "Путь к файлу конфигурации (например, ./configs/tags.yaml)")
	flag.StringVar(&logDir, "logs", "logs", "Путь к директории логов (например, ./logs или /var/log/plc_tsdb)")
	flag.Parse()

	useFileLogging := configPath != "" // если config не задан → логи только в stdout/stderr
	logFile := initLogger(logDir, useFileLogging)
	if logFile != nil {
		defer logFile.Close()
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Ошибка определения пути: %v", err)
	}
	exeDir = filepath.Dir(exePath)

	// Если путь к конфигу не задан — пробуем стандартные
	if configPath == "" {
		possiblePaths := []string{
			filepath.Join(exeDir, "configs", "tags.yaml"),
			filepath.Join(exeDir, "..", "configs", "tags.yaml"),
			"configs/tags.yaml",
			"../../configs/tags.yaml",
			"C:/pls_tsdb/configs/tags.yaml",
		}

		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}

		if configPath == "" {
			logError("Файл конфигурации не найден. Укажите путь через параметр -config")
			os.Exit(1)
		}
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logError("Ошибка загрузки конфигурации (%s): %v", configPath, err)
		os.Exit(1)
	}

	log.Printf("[INFO] Конфиг загружен: %s", configPath)

	// Создаём необходимые каталоги
	if err := os.MkdirAll("data", 0755); err != nil {
		logError("Ошибка создания каталога data: %v", err)
	}

	collector, err := service.NewCollectorService(cfg)
	if err != nil {
		logError("Ошибка создания коллектора: %v", err)
		os.Exit(1)
	}

	log.Println("[INFO] Запуск PLC Data Collector сервиса...")
	if err := collector.Start(); err != nil {
		logError("Ошибка при работе коллектора: %v", err)
		os.Exit(1)
	}

	log.Println("[INFO] Сервис остановлен")
}
