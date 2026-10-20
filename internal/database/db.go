package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"announcer/internal/paths"

	_ "modernc.org/sqlite"
)

var (
	dbConn *sql.DB
	dbMu   sync.Mutex
)

type StaticSchedule struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Time         string   `json:"time"`
	Days         []string `json:"days"`
	AudioFile    string   `json:"audio_file"`
	Volume       *float64 `json:"volume"`
	SkipHolidays bool     `json:"skip_holidays"`
	Enabled      bool     `json:"enabled"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type Holiday struct {
	ID   int64  `json:"id"`
	Date string `json:"date"`
	Name string `json:"name"`
}

type ActivityLog struct {
	ID          int64   `json:"id"`
	Timestamp   string  `json:"timestamp"`
	EventType   string  `json:"event_type"`
	Source      *string `json:"source"`
	AudioFile   *string `json:"audio_file"`
	AudioDevice *string `json:"audio_device"`
	Status      string  `json:"status"`
	Message     *string `json:"message"`
}

func InitDB() error {
	dbMu.Lock()
	defer dbMu.Unlock()

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", paths.DBFile)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer pattern
	dbConn = db

	// Create tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS static_schedules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			time TEXT NOT NULL,
			days TEXT NOT NULL,
			audio_file TEXT NOT NULL,
			volume REAL DEFAULT NULL,
			skip_holidays INTEGER DEFAULT 1,
			enabled INTEGER DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS prayer_configuration (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS holidays (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL UNIQUE,
			name TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS activity_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL,
			event_type TEXT NOT NULL,
			source TEXT,
			audio_file TEXT,
			audio_device TEXT,
			status TEXT NOT NULL,
			message TEXT
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("error creating schema: %w", err)
		}
	}

	// Seed prayer config if empty
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM prayer_configuration").Scan(&count)
	if count == 0 {
		defaults := map[string]any{
			"enabled":                  true,
			"city_id":                  "1301",
			"city_name":                "Kota Jakarta",
			"latitude":                 -6.2088,
			"longitude":                106.8456,
			"ihtiyat_minutes":          2,
			"reminder_offset_minutes":  2,
			"pre_alert_enabled":        true,
			"pre_alert_offset_minutes": 10,
			"pre_alert_audio_file":     "pre_adzan.mp3",
			"dzuhur_enabled":           true,
			"dzuhur_audio_file":        "adzan_dzuhur.mp3",
			"dzuhur_volume":            nil,
			"ashar_enabled":            true,
			"ashar_audio_file":         "adzan_ashar.mp3",
			"ashar_volume":             nil,
			"maghrib_enabled":          true,
			"maghrib_audio_file":       "adzan_maghrib.mp3",
			"maghrib_volume":           nil,
			"skip_days":                []string{"saturday", "sunday"},
		}
		for k, v := range defaults {
			b, _ := json.Marshal(v)
			_, _ = db.Exec("INSERT INTO prayer_configuration (key, value) VALUES (?, ?)", k, string(b))
		}
	}

	// Seed static schedules if empty
	_ = db.QueryRow("SELECT COUNT(*) FROM static_schedules").Scan(&count)
	if count == 0 {
		now := time.Now().Format(time.RFC3339)
		defaultDays, _ := json.Marshal([]string{"monday", "tuesday", "wednesday", "thursday", "friday"})
		istDays, _ := json.Marshal([]string{"monday", "tuesday", "wednesday", "thursday"})

		samples := []struct {
			id, name, timeStr, days, audio string
		}{
			{"sch-01", "Jam Masuk Kantor", "07:30", string(defaultDays), "masuk_kantor.mp3"},
			{"sch-02", "Istirahat Siang", "12:00", string(istDays), "istirahat.mp3"},
			{"sch-03", "Jam Pulang Kerja", "16:30", string(defaultDays), "pulang_kantor.mp3"},
		}

		for _, s := range samples {
			_, _ = db.Exec(`
				INSERT INTO static_schedules (id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, NULL, 1, 1, ?, ?)
			`, s.id, s.name, s.timeStr, s.days, s.audio, now, now)
		}
	}

	return nil
}

func GetAllSchedules() ([]StaticSchedule, error) {
	rows, err := dbConn.Query("SELECT id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at FROM static_schedules ORDER BY time ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []StaticSchedule
	for rows.Next() {
		var s StaticSchedule
		var daysStr string
		var skipHolidaysInt, enabledInt int
		if err := rows.Scan(&s.ID, &s.Name, &s.Time, &daysStr, &s.AudioFile, &s.Volume, &skipHolidaysInt, &enabledInt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(daysStr), &s.Days)
		s.SkipHolidays = skipHolidaysInt == 1
		s.Enabled = enabledInt == 1
		result = append(result, s)
	}
	if result == nil {
		result = []StaticSchedule{}
	}
	return result, nil
}

func GetScheduleByID(id string) (*StaticSchedule, error) {
	row := dbConn.QueryRow("SELECT id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at FROM static_schedules WHERE id = ?", id)
	var s StaticSchedule
	var daysStr string
	var skipHolidaysInt, enabledInt int
	if err := row.Scan(&s.ID, &s.Name, &s.Time, &daysStr, &s.AudioFile, &s.Volume, &skipHolidaysInt, &enabledInt, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(daysStr), &s.Days)
	s.SkipHolidays = skipHolidaysInt == 1
	s.Enabled = enabledInt == 1
	return &s, nil
}

func CreateSchedule(s StaticSchedule) (*StaticSchedule, error) {
	now := time.Now().Format(time.RFC3339)
	if s.ID == "" {
		s.ID = fmt.Sprintf("sch-%d", time.Now().UnixMilli())
	}
	if len(s.Days) == 0 {
		s.Days = []string{"monday", "tuesday", "wednesday", "thursday", "friday"}
	}
	daysJSON, _ := json.Marshal(s.Days)
	skipH := 0
	if s.SkipHolidays {
		skipH = 1
	}
	en := 0
	if s.Enabled {
		en = 1
	}

	_, err := dbConn.Exec(`
		INSERT INTO static_schedules (id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.Name, s.Time, string(daysJSON), s.AudioFile, s.Volume, skipH, en, now, now)
	if err != nil {
		return nil, err
	}
	return GetScheduleByID(s.ID)
}

func UpdateSchedule(id string, updates map[string]any) (*StaticSchedule, error) {
	cur, err := GetScheduleByID(id)
	if err != nil || cur == nil {
		return nil, err
	}

	if v, ok := updates["name"].(string); ok && v != "" {
		cur.Name = v
	}
	if v, ok := updates["time"].(string); ok && v != "" {
		cur.Time = v
	}
	if v, ok := updates["audio_file"].(string); ok && v != "" {
		cur.AudioFile = v
	}
	if v, ok := updates["volume"]; ok {
		if v == nil {
			cur.Volume = nil
		} else if f, ok := v.(float64); ok {
			cur.Volume = &f
		}
	}
	if v, ok := updates["skip_holidays"].(bool); ok {
		cur.SkipHolidays = v
	}
	if v, ok := updates["enabled"].(bool); ok {
		cur.Enabled = v
	}
	if v, ok := updates["days"].([]any); ok {
		var days []string
		for _, d := range v {
			if ds, ok := d.(string); ok {
				days = append(days, ds)
			}
		}
		cur.Days = days
	}

	now := time.Now().Format(time.RFC3339)
	daysJSON, _ := json.Marshal(cur.Days)
	skipH := 0
	if cur.SkipHolidays {
		skipH = 1
	}
	en := 0
	if cur.Enabled {
		en = 1
	}

	_, err = dbConn.Exec(`
		UPDATE static_schedules
		SET name = ?, time = ?, days = ?, audio_file = ?, volume = ?, skip_holidays = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, cur.Name, cur.Time, string(daysJSON), cur.AudioFile, cur.Volume, skipH, en, now, id)
	if err != nil {
		return nil, err
	}
	return GetScheduleByID(id)
}

func ToggleSchedule(id string) (*StaticSchedule, error) {
	cur, err := GetScheduleByID(id)
	if err != nil || cur == nil {
		return nil, err
	}
	return UpdateSchedule(id, map[string]any{"enabled": !cur.Enabled})
}

func DeleteSchedule(id string) error {
	_, err := dbConn.Exec("DELETE FROM static_schedules WHERE id = ?", id)
	return err
}

func GetPrayerConfig() (map[string]any, error) {
	rows, err := dbConn.Query("SELECT key, value FROM prayer_configuration")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]any)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		var parsed any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			result[k] = parsed
		} else {
			result[k] = v
		}
	}
	return result, nil
}

func UpdatePrayerConfig(updates map[string]any) (map[string]any, error) {
	for k, v := range updates {
		b, err := json.Marshal(v)
		if err != nil {
			continue
		}
		_, _ = dbConn.Exec(`
			INSERT INTO prayer_configuration (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value
		`, k, string(b))
	}
	return GetPrayerConfig()
}

func GetHolidays() ([]Holiday, error) {
	rows, err := dbConn.Query("SELECT id, date, name FROM holidays ORDER BY date ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Holiday
	for rows.Next() {
		var h Holiday
		var name sql.NullString
		if err := rows.Scan(&h.ID, &h.Date, &name); err != nil {
			continue
		}
		if name.Valid {
			h.Name = name.String
		}
		result = append(result, h)
	}
	if result == nil {
		result = []Holiday{}
	}
	return result, nil
}

func IsHoliday(dateStr string) bool {
	var count int
	_ = dbConn.QueryRow("SELECT COUNT(*) FROM holidays WHERE date = ?", dateStr).Scan(&count)
	return count > 0
}

func AddHoliday(dateStr, name string) error {
	_, err := dbConn.Exec("INSERT OR REPLACE INTO holidays (date, name) VALUES (?, ?)", dateStr, name)
	return err
}

func DeleteHoliday(id int64) error {
	_, err := dbConn.Exec("DELETE FROM holidays WHERE id = ?", id)
	return err
}

func LogActivity(eventType, source, audioFile, audioDevice, status, message string) {
	now := time.Now().Format(time.RFC3339)
	var src, af, ad, msg *string
	if source != "" {
		src = &source
	}
	if audioFile != "" {
		af = &audioFile
	}
	if audioDevice != "" {
		ad = &audioDevice
	}
	if message != "" {
		msg = &message
	}

	_, _ = dbConn.Exec(`
		INSERT INTO activity_log (timestamp, event_type, source, audio_file, audio_device, status, message)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, now, eventType, src, af, ad, status, msg)
}

func GetRecentActivityLogs(limit int) ([]ActivityLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := dbConn.Query("SELECT id, timestamp, event_type, source, audio_file, audio_device, status, message FROM activity_log ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ActivityLog
	for rows.Next() {
		var l ActivityLog
		var src, af, ad, msg sql.NullString
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.EventType, &src, &af, &ad, &l.Status, &msg); err != nil {
			continue
		}
		if src.Valid {
			l.Source = &src.String
		}
		if af.Valid {
			l.AudioFile = &af.String
		}
		if ad.Valid {
			l.AudioDevice = &ad.String
		}
		if msg.Valid {
			l.Message = &msg.String
		}
		result = append(result, l)
	}
	if result == nil {
		result = []ActivityLog{}
	}
	return result, nil
}

func ResetAllSchedules(schedules []StaticSchedule) error {
	tx, err := dbConn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM static_schedules"); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	for _, s := range schedules {
		daysJSON, _ := json.Marshal(s.Days)
		skipH := 0
		if s.SkipHolidays {
			skipH = 1
		}
		en := 0
		if s.Enabled {
			en = 1
		}
		_, err := tx.Exec(`
			INSERT INTO static_schedules (id, name, time, days, audio_file, volume, skip_holidays, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, s.ID, s.Name, s.Time, string(daysJSON), s.AudioFile, s.Volume, skipH, en, now, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func ResetAllHolidays(holidays []Holiday) error {
	tx, err := dbConn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM holidays"); err != nil {
		return err
	}
	for _, h := range holidays {
		_, err := tx.Exec("INSERT INTO holidays (date, name) VALUES (?, ?)", h.Date, h.Name)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
