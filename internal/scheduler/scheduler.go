package scheduler

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"announcer/internal/audio"
	"announcer/internal/config"
	"announcer/internal/database"
	"announcer/internal/notify"
	"announcer/internal/prayer"
)

type NextRunInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	NextRunTime string  `json:"next_run_time"`
	Timestamp   float64 `json:"timestamp"`
}

type StatusOverview struct {
	CurrentTime     string                `json:"current_time"`
	Timezone        string                `json:"timezone"`
	Engine          audio.EngineStatus    `json:"engine"`
	NextRun         *NextRunInfo          `json:"next_run"`
	ActiveJobsCount int                   `json:"active_jobs_count"`
	TodayPrayers    prayer.PrayerSchedule `json:"today_prayers"`
}

type SchedulerService struct {
	resolver       *prayer.PrayerResolver
	mu             sync.RWMutex
	todayPrayers   prayer.PrayerSchedule
	lastTriggered  map[string]string // key: jobID -> "YYYY-MM-DD HH:mm" to prevent duplicate triggers
	lastMidnight   string
	lastHealthCheck time.Time
	stopChan       chan struct{}
}

var GlobalScheduler = NewSchedulerService()

func NewSchedulerService() *SchedulerService {
	return &SchedulerService{
		resolver:      prayer.NewPrayerResolver(),
		lastTriggered: make(map[string]string),
		stopChan:      make(chan struct{}),
	}
}

func (s *SchedulerService) Start() {
	s.ReloadJobs()
	go s.runLoop()
}

func (s *SchedulerService) Stop() {
	close(s.stopChan)
}

func (s *SchedulerService) GetLocation() *time.Location {
	settings := config.GetSettings()
	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		loc, _ = time.LoadLocation("Asia/Jakarta")
	}
	return loc
}

func (s *SchedulerService) ReloadJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	loc := s.GetLocation()
	now := time.Now().In(loc)

	s.todayPrayers = s.resolver.GetTodaySchedule(now, loc)
	s.lastMidnight = now.Format("2006-01-02")
}

func (s *SchedulerService) runLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *SchedulerService) tick() {
	loc := s.GetLocation()
	now := time.Now().In(loc)
	todayStr := now.Format("2006-01-02")
	timeHM := now.Format("15:04")
	weekday := strings.ToLower(now.Weekday().String())

	// 1. Midnight Sync (at 00:01 daily)
	if now.Hour() == 0 && now.Minute() >= 1 && s.lastMidnight != todayStr {
		s.ReloadJobs()
	}

	// 2. Periodic Health Check (every 5 mins)
	if time.Since(s.lastHealthCheck) >= 5*time.Minute {
		s.lastHealthCheck = time.Now()
		settings := config.GetSettings()
		_, _ = audio.GlobalAudioEngine.CheckDeviceHealth(settings.AudioDevice)
	}

	// Only evaluate schedules at second 0
	if now.Second() != 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings := config.GetSettings()
	isHolidayToday := database.IsHoliday(todayStr)

	// 3. Static Schedules Trigger
	schedules, err := database.GetAllSchedules()
	if err == nil {
		for _, sch := range schedules {
			if !sch.Enabled || sch.Time != timeHM {
				continue
			}

			// Check day match
			dayMatch := false
			for _, d := range sch.Days {
				if strings.ToLower(d) == weekday {
					dayMatch = true
					break
				}
			}
			if !dayMatch {
				continue
			}

			triggerKey := fmt.Sprintf("static_%s", sch.ID)
			curMinuteStr := fmt.Sprintf("%s %s", todayStr, timeHM)
			if s.lastTriggered[triggerKey] == curMinuteStr {
				continue
			}
			s.lastTriggered[triggerKey] = curMinuteStr

			// Holiday check
			if sch.SkipHolidays && isHolidayToday {
				database.LogActivity("playback", sch.Name, sch.AudioFile, "", "skipped", "Dilewati (Hari Libur)")
				continue
			}

			notify.ShowNotification("Pengumuman Otomatis", fmt.Sprintf("Memutar pengumuman: %s", sch.Name))
			audio.GlobalAudioEngine.PlayScheduled(
				sch.AudioFile,
				sch.Volume,
				settings.PrependChime,
				sch.Name,
				settings.AudioDevice,
			)
		}
	}

	// 4. Dynamic Prayer Reminders Trigger
	if s.todayPrayers.Enabled {
		cfg := s.todayPrayers.Config
		skipDaysRaw, _ := cfg["skip_days"].([]any)
		isSkipDay := false
		for _, d := range skipDaysRaw {
			if strings.ToLower(fmt.Sprintf("%v", d)) == weekday {
				isSkipDay = true
				break
			}
		}

		if !isSkipDay {
			for _, prayerName := range []string{"dzuhur", "ashar", "maghrib"} {
				enabledKey := fmt.Sprintf("%s_enabled", prayerName)
				if en, ok := cfg[enabledKey].(bool); !ok || !en {
					continue
				}

				reminderTime := s.todayPrayers.Reminders[prayerName]
				if reminderTime == "" || reminderTime != timeHM {
					continue
				}

				triggerKey := fmt.Sprintf("prayer_%s", prayerName)
				curMinuteStr := fmt.Sprintf("%s %s", todayStr, timeHM)
				if s.lastTriggered[triggerKey] == curMinuteStr {
					continue
				}
				s.lastTriggered[triggerKey] = curMinuteStr

				audioFile := "pre_adzan.mp3"
				if af, ok := cfg[fmt.Sprintf("%s_audio_file", prayerName)].(string); ok && af != "" {
					audioFile = af
				}

				var vol *float64
				if v, ok := cfg[fmt.Sprintf("%s_volume", prayerName)].(float64); ok {
					vol = &v
				}

				displayName := strings.ToUpper(prayerName[:1]) + prayerName[1:]
				title := fmt.Sprintf("Pengingat Menjelang Waktu Sholat %s", displayName)
				notify.ShowNotification("Pengingat Sholat", fmt.Sprintf("2 Menit Menjelang Waktu Sholat %s", displayName))

				audio.GlobalAudioEngine.PlayScheduled(
					audioFile,
					vol,
					settings.PrependChime,
					title,
					settings.AudioDevice,
				)
			}
		}
	}
}

func (s *SchedulerService) GetNextRun() *NextRunInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	loc := s.GetLocation()
	now := time.Now().In(loc)

	type candidateJob struct {
		id        string
		name      string
		targetTime time.Time
	}

	var candidates []candidateJob

	// 1. Static schedules over next 7 days
	schedules, _ := database.GetAllSchedules()
	for dayOffset := 0; dayOffset <= 7; dayOffset++ {
		checkDay := now.AddDate(0, 0, dayOffset)
		checkDayStr := checkDay.Format("2006-01-02")
		checkWeekday := strings.ToLower(checkDay.Weekday().String())
		isHoliday := database.IsHoliday(checkDayStr)

		for _, sch := range schedules {
			if !sch.Enabled {
				continue
			}
			if sch.SkipHolidays && isHoliday {
				continue
			}
			matchDay := false
			for _, d := range sch.Days {
				if strings.ToLower(d) == checkWeekday {
					matchDay = true
					break
				}
			}
			if !matchDay {
				continue
			}

			parts := strings.Split(sch.Time, ":")
			if len(parts) != 2 {
				continue
			}
			var h, m int
			_, _ = fmt.Sscanf(parts[0], "%d", &h)
			_, _ = fmt.Sscanf(parts[1], "%d", &m)

			target := time.Date(checkDay.Year(), checkDay.Month(), checkDay.Day(), h, m, 0, 0, loc)
			if target.After(now) {
				candidates = append(candidates, candidateJob{
					id:        fmt.Sprintf("static_%s", sch.ID),
					name:      sch.Name,
					targetTime: target,
				})
			}
		}

		// 2. Prayer reminders for today & tomorrow
		if s.todayPrayers.Enabled && dayOffset <= 1 {
			cfg := s.todayPrayers.Config
			for _, p := range []string{"dzuhur", "ashar", "maghrib"} {
				if en, ok := cfg[p+"_enabled"].(bool); ok && !en {
					continue
				}
				remTime := s.todayPrayers.Reminders[p]
				if remTime == "" {
					continue
				}
				var h, m int
				_, _ = fmt.Sscanf(remTime, "%d:%d", &h, &m)
				target := time.Date(checkDay.Year(), checkDay.Month(), checkDay.Day(), h, m, 0, 0, loc)
				if target.After(now) {
					displayName := strings.ToUpper(p[:1]) + p[1:]
					candidates = append(candidates, candidateJob{
						id:        fmt.Sprintf("prayer_%s", p),
						name:      fmt.Sprintf("Pengingat Sholat %s", displayName),
						targetTime: target,
					})
				}
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].targetTime.Before(candidates[j].targetTime)
	})

	earliest := candidates[0]
	return &NextRunInfo{
		ID:          earliest.id,
		Name:        earliest.name,
		NextRunTime: earliest.targetTime.Format(time.RFC3339),
		Timestamp:   float64(earliest.targetTime.Unix()),
	}
}

func (s *SchedulerService) GetStatusOverview() StatusOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	loc := s.GetLocation()
	now := time.Now().In(loc)
	settings := config.GetSettings()

	schedules, _ := database.GetAllSchedules()
	activeCount := 0
	for _, sch := range schedules {
		if sch.Enabled {
			activeCount++
		}
	}
	if s.todayPrayers.Enabled {
		activeCount += 3 // Dzuhur, Ashar, Maghrib
	}

	return StatusOverview{
		CurrentTime:     now.Format("2006-01-02 15:04:05"),
		Timezone:        settings.Timezone,
		Engine:          audio.GlobalAudioEngine.GetStatus(),
		NextRun:         s.GetNextRun(),
		ActiveJobsCount: activeCount,
		TodayPrayers:    s.todayPrayers,
	}
}
