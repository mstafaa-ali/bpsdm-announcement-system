package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"announcer/internal/audio"
	"announcer/internal/autostart"
	"announcer/internal/config"
	"announcer/internal/database"
	"announcer/internal/paths"
	"announcer/internal/scheduler"
)

var allowedAudioExts = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".ogg":  true,
	".flac": true,
	".m4a":  true,
	".aac":  true,
}

type Server struct {
	mux        *http.ServeMux
	embeddedFS fs.FS
}

func NewServer(embeddedFS fs.FS) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		embeddedFS: embeddedFS,
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	// Web dashboard root
	s.mux.HandleFunc("/", s.handleIndex)

	// Status & System
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/devices", s.handleDevices)
	s.mux.HandleFunc("/api/settings", s.handleSettings)

	// Static Schedules
	s.mux.HandleFunc("/api/schedules", s.handleSchedules)
	s.mux.HandleFunc("/api/schedules/", s.handleScheduleItem) // Item CRUD & toggle

	// Prayer Configuration
	s.mux.HandleFunc("/api/prayer", s.handlePrayer)
	s.mux.HandleFunc("/api/prayer/today", s.handlePrayerToday)

	// Media Manager
	s.mux.HandleFunc("/api/media", s.handleMediaList)
	s.mux.HandleFunc("/api/media/upload", s.handleMediaUpload)
	s.mux.HandleFunc("/api/media/stream/", s.handleMediaStream)
	s.mux.HandleFunc("/api/media/", s.handleMediaDelete)

	// Audio Controls
	s.mux.HandleFunc("/api/audio/test", s.handleAudioTest)
	s.mux.HandleFunc("/api/audio/emergency", s.handleAudioEmergency)
	s.mux.HandleFunc("/api/audio/stop", s.handleAudioStop)

	// Logs & Backup
	s.mux.HandleFunc("/api/logs", s.handleLogs)
	s.mux.HandleFunc("/api/backup/export", s.handleBackupExport)
	s.mux.HandleFunc("/api/backup/import", s.handleBackupImport)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// Try serving from static files if exists
		staticDiskPath := filepath.Join(paths.AppDir, r.URL.Path)
		if _, err := os.Stat(staticDiskPath); err == nil {
			http.ServeFile(w, r, staticDiskPath)
			return
		}
		http.NotFound(w, r)
		return
	}

	// 1. Try disk file first
	diskPath := filepath.Join(paths.AppDir, "templates", "index.html")
	if data, err := os.ReadFile(diskPath); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
		return
	}

	// 2. Fallback to embedded
	if s.embeddedFS != nil {
		if data, err := fs.ReadFile(s.embeddedFS, "templates/index.html"); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(data)
			return
		}
	}

	http.Error(w, "index.html not found", http.StatusNotFound)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	overview := scheduler.GlobalScheduler.GetStatusOverview()
	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status": "success",
		"data":   overview,
	})
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices := audio.GlobalAudioEngine.GetOutputDevices()
	settings := config.GetSettings()
	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status":         "success",
		"devices":        devices,
		"current_device": settings.AudioDevice,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":   "success",
			"settings": config.GetSettings(),
		})
		return
	}

	if r.Method == http.MethodPost {
		var updates map[string]any
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			s.jsonError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		updated, err := config.UpdateSettings(updates)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if autoStart, ok := updates["auto_start_on_boot"].(bool); ok {
			autostart.SetAutoStart(autoStart)
		}

		scheduler.GlobalScheduler.ReloadJobs()
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":   "success",
			"settings": updated,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		schedules, err := database.GetAllSchedules()
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":    "success",
			"schedules": schedules,
		})
		return
	}

	if r.Method == http.MethodPost {
		var input struct {
			Name         string   `json:"name"`
			Time         string   `json:"time"`
			Days         []string `json:"days"`
			AudioFile    string   `json:"audio_file"`
			Volume       *float64 `json:"volume"`
			SkipHolidays bool     `json:"skip_holidays"`
			Enabled      bool     `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			s.jsonError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		if input.Name == "" || input.Time == "" || input.AudioFile == "" {
			s.jsonError(w, http.StatusBadRequest, "Nama, jam, dan file audio wajib diisi!")
			return
		}

		newSch, err := database.CreateSchedule(database.StaticSchedule{
			Name:         input.Name,
			Time:         input.Time,
			Days:         input.Days,
			AudioFile:    input.AudioFile,
			Volume:       input.Volume,
			SkipHolidays: input.SkipHolidays,
			Enabled:      input.Enabled,
		})
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		scheduler.GlobalScheduler.ReloadJobs()
		database.LogActivity("system", "web_dashboard", "", "", "success", fmt.Sprintf("Created schedule '%s'", input.Name))

		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":   "success",
			"schedule": newSch,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleScheduleItem(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/schedules/")
	parts := strings.Split(path, "/")
	id := parts[0]

	if id == "" {
		http.NotFound(w, r)
		return
	}

	// Toggle endpoint: /api/schedules/{id}/toggle
	if len(parts) >= 2 && parts[1] == "toggle" && r.Method == http.MethodPost {
		updated, err := database.ToggleSchedule(id)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if updated == nil {
			s.jsonError(w, http.StatusNotFound, "Jadwal tidak ditemukan")
			return
		}

		scheduler.GlobalScheduler.ReloadJobs()
		state := "Inactive"
		if updated.Enabled {
			state = "Active"
		}
		database.LogActivity("system", "web_dashboard", "", "", "success", fmt.Sprintf("Toggled schedule '%s' to %s", updated.Name, state))

		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":   "success",
			"schedule": updated,
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		sch, err := database.GetScheduleByID(id)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sch == nil {
			s.jsonError(w, http.StatusNotFound, "Jadwal tidak ditemukan")
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":   "success",
			"schedule": sch,
		})

	case http.MethodPut:
		var updates map[string]any
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			s.jsonError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		updated, err := database.UpdateSchedule(id, updates)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if updated == nil {
			s.jsonError(w, http.StatusNotFound, "Jadwal tidak ditemukan")
			return
		}

		scheduler.GlobalScheduler.ReloadJobs()
		database.LogActivity("system", "web_dashboard", "", "", "success", fmt.Sprintf("Updated schedule '%s'", updated.Name))

		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":   "success",
			"schedule": updated,
		})

	case http.MethodDelete:
		err := database.DeleteSchedule(id)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		scheduler.GlobalScheduler.ReloadJobs()
		database.LogActivity("system", "web_dashboard", "", "", "success", fmt.Sprintf("Deleted schedule %s", id))
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":  "success",
			"message": "Jadwal berhasil dihapus",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePrayer(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg, err := database.GetPrayerConfig()
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status": "success",
			"config": cfg,
		})
		return
	}

	if r.Method == http.MethodPost {
		var updates map[string]any
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			s.jsonError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}
		updated, err := database.UpdatePrayerConfig(updates)
		if err != nil {
			s.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		scheduler.GlobalScheduler.ReloadJobs()
		database.LogActivity("system", "web_dashboard", "", "", "success", "Updated prayer configurations")
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status": "success",
			"config": updated,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handlePrayerToday(w http.ResponseWriter, r *http.Request) {
	overview := scheduler.GlobalScheduler.GetStatusOverview()
	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status": "success",
		"data":   overview.TodayPrayers,
	})
}

func (s *Server) handleMediaList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type MediaItem struct {
		Filename  string  `json:"filename"`
		SizeBytes int64   `json:"size_bytes"`
		SizeMB    float64 `json:"size_mb"`
		Modified  string  `json:"modified"`
	}

	var items []MediaItem
	entries, err := os.ReadDir(paths.MediaDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if !allowedAudioExts[ext] {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			size := info.Size()
			items = append(items, MediaItem{
				Filename:  e.Name(),
				SizeBytes: size,
				SizeMB:    float64(size) / (1024 * 1024),
				Modified:  info.ModTime().Format(time.RFC3339),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Filename) < strings.ToLower(items[j].Filename)
	})

	if items == nil {
		items = []MediaItem{}
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status": "success",
		"files":  items,
	})
}

func (s *Server) handleMediaUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 50 MB max
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		s.jsonError(w, http.StatusBadRequest, "File terlalu besar (maksimal 50 MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "Tidak ada file yang diunggah")
		return
	}
	defer file.Close()

	cleanName := filepath.Base(header.Filename)
	ext := strings.ToLower(filepath.Ext(cleanName))
	if !allowedAudioExts[ext] {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("Format audio '%s' tidak didukung. Gunakan MP3, WAV, atau OGG.", ext))
		return
	}

	dstPath := filepath.Join(paths.MediaDir, cleanName)
	dst, err := os.Create(dstPath)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	database.LogActivity("system", "web_dashboard", cleanName, "", "success", "Audio file uploaded")
	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status":   "success",
		"filename": cleanName,
	})
}

func (s *Server) handleMediaDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/media/"))
	targetPath := filepath.Join(paths.MediaDir, filename)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		s.jsonError(w, http.StatusNotFound, "File tidak ditemukan")
		return
	}

	if err := os.Remove(targetPath); err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	database.LogActivity("system", "web_dashboard", filename, "", "success", "Audio file deleted")
	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "File audio berhasil dihapus",
	})
}

func (s *Server) handleMediaStream(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(strings.TrimPrefix(r.URL.Path, "/api/media/stream/"))
	targetPath := filepath.Join(paths.MediaDir, filename)

	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, targetPath)
}

func (s *Server) handleAudioTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		FileName     string   `json:"file_name"`
		Volume       *float64 `json:"volume"`
		PrependChime bool     `json:"prepend_chime"`
		Device       *string  `json:"device"`
	}
	_ = json.NewDecoder(r.Body).Decode(&data)

	if data.FileName == "" {
		data.FileName = "chime.mp3"
	}

	targetFile := filepath.Join(paths.MediaDir, data.FileName)
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		s.jsonError(w, http.StatusNotFound, fmt.Sprintf("File %s tidak ditemukan di folder media", data.FileName))
		return
	}

	audio.GlobalAudioEngine.PlayScheduled(
		data.FileName,
		data.Volume,
		data.PrependChime,
		"Manual Test",
		data.Device,
	)

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("Memutar '%s'...", data.FileName),
	})
}

func (s *Server) handleAudioEmergency(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		FileName string `json:"file_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&data)
	if data.FileName == "" {
		data.FileName = "emergency.mp3"
	}

	audio.GlobalAudioEngine.PlayEmergency(data.FileName)
	database.LogActivity("playback", "Emergency Broadcast", data.FileName, "", "success", "EMERGENCY BROADCAST TRIGGERED!")

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Peringatan darurat (Emergency Broadcast) diaktifkan!",
	})
}

func (s *Server) handleAudioStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	success := audio.GlobalAudioEngine.Stop()
	if success {
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"status":  "success",
			"message": "Pemutaran audio berhasil dihentikan.",
		})
		return
	}
	s.jsonError(w, http.StatusInternalServerError, "Gagal menghentikan audio")
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	logs, err := database.GetRecentActivityLogs(limit)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status": "success",
		"logs":   logs,
	})
}

func (s *Server) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	prayerCfg, _ := database.GetPrayerConfig()
	schedules, _ := database.GetAllSchedules()
	holidays, _ := database.GetHolidays()

	backupData := map[string]any{
		"version":              "2.0.0",
		"exported_at":          time.Now().Format(time.RFC3339),
		"system_settings":      config.GetSettings(),
		"prayer_configuration": prayerCfg,
		"static_schedules":     schedules,
		"holidays":             holidays,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=announcer_backup_%s.json", time.Now().Format("2006-01-02")))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(backupData)
}

func (s *Server) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, http.StatusBadRequest, "Tidak ada file backup yang diunggah")
		return
	}
	defer file.Close()

	var content struct {
		SystemSettings      map[string]any            `json:"system_settings"`
		PrayerConfiguration map[string]any            `json:"prayer_configuration"`
		StaticSchedules     []database.StaticSchedule `json:"static_schedules"`
		Holidays            []database.Holiday        `json:"holidays"`
	}

	if err := json.NewDecoder(file).Decode(&content); err != nil {
		s.jsonError(w, http.StatusBadRequest, fmt.Sprintf("Gagal membaca backup: %v", err))
		return
	}

	if content.SystemSettings != nil {
		_, _ = config.UpdateSettings(content.SystemSettings)
	}
	if content.PrayerConfiguration != nil {
		_, _ = database.UpdatePrayerConfig(content.PrayerConfiguration)
	}
	if content.StaticSchedules != nil {
		_ = database.ResetAllSchedules(content.StaticSchedules)
	}
	if content.Holidays != nil {
		_ = database.ResetAllHolidays(content.Holidays)
	}

	scheduler.GlobalScheduler.ReloadJobs()
	database.LogActivity("system", "web_dashboard", "", "", "success", "Full configuration restored from backup")

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": "Konfigurasi berhasil dipulihkan dari file backup!",
	})
}

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]any{
		"status":  "error",
		"message": message,
	})
}
