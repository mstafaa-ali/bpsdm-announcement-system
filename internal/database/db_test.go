package database

import (
	"os"
	"path/filepath"
	"testing"

	"announcer/internal/paths"
)

func setupTestDB(t *testing.T) func() {
	tmpDir, err := os.MkdirTemp("", "announcer-db-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	oldDB := paths.DBFile
	paths.DBFile = filepath.Join(tmpDir, "test.db")

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	return func() {
		if dbConn != nil {
			_ = dbConn.Close()
		}
		paths.DBFile = oldDB
		_ = os.RemoveAll(tmpDir)
	}
}

func TestDatabaseOperations(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Verify default schedules exist after init
	schedules, err := GetAllSchedules()
	if err != nil {
		t.Fatalf("GetAllSchedules error: %v", err)
	}
	if len(schedules) == 0 {
		t.Errorf("Expected initial seeded schedules, got 0")
	}

	// 2. Create schedule
	sch, err := CreateSchedule(StaticSchedule{
		Name:      "Test Meeting",
		Time:      "10:00",
		Days:      []string{"monday"},
		AudioFile: "chime.mp3",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateSchedule error: %v", err)
	}
	if sch.Name != "Test Meeting" {
		t.Errorf("Expected 'Test Meeting', got %s", sch.Name)
	}

	// 3. Toggle schedule
	toggled, err := ToggleSchedule(sch.ID)
	if err != nil {
		t.Fatalf("ToggleSchedule error: %v", err)
	}
	if toggled.Enabled != false {
		t.Errorf("Expected toggled.Enabled = false, got true")
	}

	// 4. Update schedule
	updated, err := UpdateSchedule(sch.ID, map[string]any{"name": "Updated Meeting"})
	if err != nil {
		t.Fatalf("UpdateSchedule error: %v", err)
	}
	if updated.Name != "Updated Meeting" {
		t.Errorf("Expected 'Updated Meeting', got %s", updated.Name)
	}

	// 5. Delete schedule
	err = DeleteSchedule(sch.ID)
	if err != nil {
		t.Fatalf("DeleteSchedule error: %v", err)
	}
	deleted, err := GetScheduleByID(sch.ID)
	if err != nil {
		t.Fatalf("GetScheduleByID error: %v", err)
	}
	if deleted != nil {
		t.Errorf("Expected schedule to be deleted, but still found")
	}

	// 6. Test Holidays
	if err := AddHoliday("2026-12-25", "Hari Natal"); err != nil {
		t.Fatalf("AddHoliday error: %v", err)
	}
	if !IsHoliday("2026-12-25") {
		t.Errorf("Expected 2026-12-25 to be recognized as holiday")
	}

	holidays, err := GetHolidays()
	if err != nil || len(holidays) == 0 {
		t.Fatalf("GetHolidays failed: %v", err)
	}
	if err := DeleteHoliday(holidays[0].ID); err != nil {
		t.Fatalf("DeleteHoliday error: %v", err)
	}
	if IsHoliday("2026-12-25") {
		t.Errorf("Expected 2026-12-25 to be deleted")
	}

	// 7. Activity Log
	LogActivity("test_event", "test_source", "test.mp3", "default", "success", "Log test message")
	logs, err := GetRecentActivityLogs(10)
	if err != nil {
		t.Fatalf("GetRecentActivityLogs error: %v", err)
	}
	if len(logs) == 0 {
		t.Errorf("Expected activity logs to have at least 1 entry")
	}
}
