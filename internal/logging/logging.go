package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	Logger *slog.Logger
	once   sync.Once
)

// Init инициализирует глобальный логгер slog.
// Если logDir == "" → логи только в stdout/stderr.
// Если logDir задан → логи дублируются в stdout + файл.
func Init(logDir string) error {
	var err error
	once.Do(func() {
		var handler slog.Handler

		// Базовый обработчик (stdout)
		opts := &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelInfo,
		}

		if logDir == "" {
			handler = slog.NewTextHandler(os.Stdout, opts)
			Logger = slog.New(handler)
			return
		}

		// Если logDir указан → создаём файл
		os.MkdirAll(logDir, 0755)
		filename := filepath.Join(logDir, time.Now().Format("2006-01-02_15-04-05")+".log")

		file, ferr := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if ferr != nil {
			err = fmt.Errorf("не удалось создать файл логов: %v", ferr)
			handler = slog.NewTextHandler(os.Stdout, opts)
			Logger = slog.New(handler)
			return
		}

		// Дублируем вывод в stdout + файл
		mw := io.MultiWriter(os.Stdout, file)
		handler = slog.NewTextHandler(mw, opts)
		Logger = slog.New(handler)
	})

	return err
}

// Упрощённые функции
func Info(msg string, args ...any)  { Logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { Logger.Warn(msg, args...) }
func Error(msg string, args ...any) { Logger.Error(msg, args...) }
func Debug(msg string, args ...any) { Logger.Debug(msg, args...) }

// func Fatal(msg string, args ...any) { Logger.Fatal(msg, args...) }
