package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"plc_tsdb/internal/config"

	_ "modernc.org/sqlite"
)

type SQLiteClient struct {
	db     *sql.DB
	config *config.DatabaseConfig
}

// NumericData представляет числовые данные для ИНС
type NumericData struct {
	Timestamp int64
	TagName   string
	Value     float64 // Универсальное числовое представление
	Quality   int     // 0=good, 1=bad (ошибка чтения)
}

func NewSQLiteClient(cfg *config.DatabaseConfig) (*SQLiteClient, error) {
	if err := os.MkdirAll(cfg.Database, 0755); err != nil {
		return nil, fmt.Errorf("ошибка создания директории: %w", err)
	}

	dbPath := filepath.Join(cfg.Database, "plc_data.db")

	// Оптимизация для временных рядов и параллельного доступа
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_sync=NORMAL&_cache=shared&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %w", err)
	}

	// Настройки для параллельных читателей (ИНС)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	client := &SQLiteClient{
		db:     db,
		config: cfg,
	}

	if err := client.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка инициализации схемы: %w", err)
	}

	log.Printf("SQLite база данных создана: %s", dbPath)
	return client, nil
}

func (s *SQLiteClient) initSchema() error {
	// Упрощенная таблица только для числовых данных
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS numeric_time_series (
			timestamp_ns INTEGER NOT NULL,
			tag_name TEXT NOT NULL,
			value REAL NOT NULL,        -- Только числовые значения
			quality INTEGER DEFAULT 0,  -- 0=good, 1=bad
			PRIMARY KEY (timestamp_ns, tag_name)
		)
	`)
	if err != nil {
		return err
	}

	// Индексы для быстрого поиска по времени и тегам
	_, err = s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_nts_timestamp ON numeric_time_series(timestamp_ns);
		CREATE INDEX IF NOT EXISTS idx_nts_tag ON numeric_time_series(tag_name);
		CREATE INDEX IF NOT EXISTS idx_nts_timestamp_tag ON numeric_time_series(timestamp_ns, tag_name);
		CREATE INDEX IF NOT EXISTS idx_nts_quality ON numeric_time_series(quality);
	`)

	return err
}

func (s *SQLiteClient) Write(data map[string]interface{}, timestamp time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	timestampNs := timestamp.UnixNano()
	successfulWrites := 0

	for tagName, value := range data {
		numericValue, quality, valid := s.convertToNumeric(value)
		if !valid {
			log.Printf("Пропуск нечислового тега %s: тип %T", tagName, value)
			continue
		}

		_, err = tx.Exec(`
			INSERT INTO numeric_time_series (timestamp_ns, tag_name, value, quality)
			VALUES (?, ?, ?, ?)
		`, timestampNs, tagName, numericValue, quality)

		if err != nil {
			log.Printf("Ошибка записи тега %s: %v", tagName, err)
		} else {
			successfulWrites++
		}
	}

	if successfulWrites == 0 {
		return fmt.Errorf("ни один тег не был записан")
	}

	return tx.Commit()
}

// convertToNumeric преобразует поддерживаемые типы в float64
func (s *SQLiteClient) convertToNumeric(value interface{}) (float64, int, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), 0, true
	case float64:
		return v, 0, true
	case int:
		return float64(v), 0, true
	case int16:
		return float64(v), 0, true
	case int32:
		return float64(v), 0, true
	case int64:
		if value, ok := safeInt64ToFloat(v); ok {
			return value, 0, true
		}
		return 0.0, 1, false
	case uint16:
		return float64(v), 0, true
	case uint32:
		return float64(v), 0, true
	case bool:
		if v {
			return 1.0, 0, true
		}
		return 0.0, 0, true
	default:
		return 0.0, 1, false // Неподдерживаемый тип
	}
}

// Методы для ИНС

// GetNumericData возвращает числовые данные для указанных тегов и временного диапазона
func (s *SQLiteClient) GetNumericData(tags []string, startTime, endTime time.Time) ([]NumericData, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("не указаны теги")
	}

	// Создаем плейсхолдеры для IN clause
	placeholders := ""
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = tag
	}

	// Добавляем временной диапазон
	args = append(args, startTime.UnixNano(), endTime.UnixNano())

	query := fmt.Sprintf(`
		SELECT timestamp_ns, tag_name, value, quality
		FROM numeric_time_series 
		WHERE tag_name IN (%s)
		AND timestamp_ns BETWEEN ? AND ?
		AND quality = 0  -- Только данные хорошего качества
		ORDER BY timestamp_ns, tag_name
	`, placeholders)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NumericData
	for rows.Next() {
		var data NumericData
		err := rows.Scan(&data.Timestamp, &data.TagName, &data.Value, &data.Quality)
		if err != nil {
			return nil, err
		}
		results = append(results, data)
	}

	return results, nil
}

// GetDataForTraining возвращает данные в формате для обучения ИНС
func (s *SQLiteClient) GetDataForTraining(tags []string, startTime, endTime time.Time) (*TrainingData, error) {
	numericData, err := s.GetNumericData(tags, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Группируем данные по временным меткам
	dataByTime := make(map[int64]map[string]float64)
	var timestamps []int64

	for _, data := range numericData {
		if _, exists := dataByTime[data.Timestamp]; !exists {
			dataByTime[data.Timestamp] = make(map[string]float64)
			timestamps = append(timestamps, data.Timestamp)
		}
		dataByTime[data.Timestamp][data.TagName] = data.Value
	}

	// Создаем матрицу признаков
	features := make([][]float64, len(timestamps))
	for i, ts := range timestamps {
		row := make([]float64, len(tags))
		for j, tag := range tags {
			if value, exists := dataByTime[ts][tag]; exists {
				row[j] = value
			} else {
				row[j] = 0.0 // Заполняем нулями пропущенные значения
			}
		}
		features[i] = row
	}

	return &TrainingData{
		Features:   features,
		Timestamps: timestamps,
		Tags:       tags,
	}, nil
}

// TrainingData структура для данных обучения ИНС
type TrainingData struct {
	Features   [][]float64 // [time][feature] матрица признаков
	Timestamps []int64     // Временные метки
	Tags       []string    // Названия тегов (признаков)
}

// GetRecentData возвращает последние N записей для указанных тегов
func (s *SQLiteClient) GetRecentData(tags []string, limit int) ([]NumericData, error) {
	if len(tags) == 0 {
		return nil, fmt.Errorf("не указаны теги")
	}

	placeholders := ""
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = tag
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT timestamp_ns, tag_name, value, quality
		FROM numeric_time_series 
		WHERE tag_name IN (%s)
		AND quality = 0
		ORDER BY timestamp_ns DESC 
		LIMIT ?
	`, placeholders)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NumericData
	for rows.Next() {
		var data NumericData
		err := rows.Scan(&data.Timestamp, &data.TagName, &data.Value, &data.Quality)
		if err != nil {
			return nil, err
		}
		results = append(results, data)
	}

	// Переворачиваем чтобы получить хронологический порядок
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	return results, nil
}

// CleanOldData удаляет данные старше указанного времени
func (s *SQLiteClient) CleanOldData(olderThan time.Time) error {
	_, err := s.db.Exec(`
		DELETE FROM numeric_time_series 
		WHERE timestamp_ns < ?
	`, olderThan.UnixNano())
	return err
}

func (s *SQLiteClient) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// safeInt64ToFloat проверяет что int64 безопасно конвертируется в float64
func safeInt64ToFloat(v int64) (float64, bool) {
	// Float64 может точно представить integers до 2^53
	if v > 1<<53 || v < -1<<53 {
		return 0, false
	}
	return float64(v), true
}
