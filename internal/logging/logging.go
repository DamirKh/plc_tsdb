package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	Logger *slog.Logger
	once   sync.Once
)

type SimpleHandler struct {
	level slog.Level
	out   *os.File
}

func NewSimpleHandler(level slog.Level, out *os.File) *SimpleHandler {
	return &SimpleHandler{level: level, out: out}
}

func (h *SimpleHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level
}

func (h *SimpleHandler) Handle(_ context.Context, r slog.Record) error {
	level := fmt.Sprintf("[%s]", r.Level.String())
	timestamp := r.Time.Format("2006-01-02T15:04:05.000Z07:00")

	// Сообщение
	msg := r.Message

	// Собираем ключи, если есть
	if r.NumAttrs() > 0 {
		r.Attrs(func(a slog.Attr) bool {
			msg += fmt.Sprintf(" %s=%v", a.Key, a.Value)
			return true
		})
	}

	fmt.Fprintf(h.out, "%s %s %s\n", timestamp, level, msg)
	return nil
}

func (h *SimpleHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *SimpleHandler) WithGroup(name string) slog.Handler       { return h }

// Init инициализирует глобальный логгер slog.
// Если logDir == "" → логи только в stdout/stderr.
// Если logDir задан → логи дублируются в stdout + файл.
// ------------------------------------------------------
// Инициализация логгера (пример)
// ------------------------------------------------------
func Init(logDir string, levelStr string) error {
	var handler slog.Handler

	// Преобразуем уровень в slog.Level
	var lvl slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return fmt.Errorf("неизвестный уровень логирования: %s", levelStr)
	}

	if logDir == "" {
		// STDOUT/STDERR режим
		handler = NewSimpleHandler(lvl, os.Stdout)
	} else {
		os.MkdirAll(logDir, 0755)
		f, err := os.OpenFile(logDir+"/plc_tsdb.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		handler = NewSimpleHandler(lvl, f)
	}

	Logger = slog.New(handler)
	slog.SetDefault(Logger)
	return nil
}

// Упрощённые функции
func Info(msg string, args ...any)  { Logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { Logger.Warn(msg, args...) }
func Error(msg string, args ...any) { Logger.Error(msg, args...) }
func Debug(msg string, args ...any) { Logger.Debug(msg, args...) }

// func Fatal(msg string, args ...any) { Logger.Fatal(msg, args...) }
