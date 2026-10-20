package prayer

import (
	"testing"
	"time"
)

func TestCalculateOffline(t *testing.T) {
	resolver := NewPrayerResolver()
	// Jakarta coordinates: Lat -6.2088, Lng 106.8456, Timezone +7
	targetDate := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	times := resolver.CalculateOffline(-6.2088, 106.8456, targetDate, 7.0)

	expectedKeys := []string{"subuh", "dzuhur", "ashar", "maghrib", "isya"}
	for _, key := range expectedKeys {
		val, ok := times[key]
		if !ok || len(val) != 5 || val[2] != ':' {
			t.Errorf("Expected valid time for %s, got %s", key, val)
		}
	}
}

func TestApplyIhtiyat(t *testing.T) {
	resolver := NewPrayerResolver()

	tests := []struct {
		input    string
		offset   int
		expected string
	}{
		{"11:58", 2, "12:00"},
		{"12:00", -2, "11:58"},
		{"00:01", -2, "23:59"},
		{"23:59", 2, "00:01"},
		{"invalid", 2, "invalid"},
	}

	for _, tt := range tests {
		got := resolver.ApplyIhtiyat(tt.input, tt.offset)
		if got != tt.expected {
			t.Errorf("ApplyIhtiyat(%s, %d) = %s, want %s", tt.input, tt.offset, got, tt.expected)
		}
	}
}

func TestCalculatePreAlert(t *testing.T) {
	resolver := NewPrayerResolver()
	got := resolver.CalculatePreAlert("12:00", 10)
	if got != "11:50" {
		t.Errorf("CalculatePreAlert('12:00', 10) = %s, want '11:50'", got)
	}
}
