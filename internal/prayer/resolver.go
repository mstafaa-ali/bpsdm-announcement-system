package prayer

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"announcer/internal/database"
)

type PrayerResolver struct {
	client *http.Client
}

type MyQuranResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Jadwal struct {
			Subuh   string `json:"subuh"`
			Dzuhur  string `json:"dzuhur"`
			Ashar   string `json:"ashar"`
			Maghrib string `json:"maghrib"`
			Isya    string `json:"isya"`
		} `json:"jadwal"`
	} `json:"data"`
}

type PrayerSchedule struct {
	Enabled   bool           `json:"enabled"`
	Date      string         `json:"date"`
	Source    string         `json:"source"`
	CityName  string         `json:"city_name"`
	Times     map[string]string `json:"times"`
	Reminders map[string]string `json:"reminders"`
	PreAlerts map[string]string `json:"pre_alerts"`
	Config    map[string]any    `json:"config"`
}

func NewPrayerResolver() *PrayerResolver {
	return &PrayerResolver{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (r *PrayerResolver) FetchFromAPI(cityID string, targetDate time.Time) (map[string]string, error) {
	url := fmt.Sprintf("https://api.myquran.com/v2/sholat/jadwal/%s/%d/%02d/%02d",
		cityID, targetDate.Year(), targetDate.Month(), targetDate.Day())

	resp, err := r.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var res MyQuranResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if !res.Status {
		return nil, fmt.Errorf("API response status false")
	}

	j := res.Data.Jadwal
	if j.Dzuhur == "" {
		return nil, fmt.Errorf("empty prayer times returned")
	}

	return map[string]string{
		"subuh":   j.Subuh,
		"dzuhur":  j.Dzuhur,
		"ashar":   j.Ashar,
		"maghrib": j.Maghrib,
		"isya":    j.Isya,
	}, nil
}

// CalculateOffline calculates astronomical prayer times according to Kemenag/MABIMS guidelines:
// Subuh: 20°, Ashar: Shafi'i (shadow factor 1), Maghrib: -0.833°, Isya: 18°.
func (r *PrayerResolver) CalculateOffline(lat, lng float64, targetDate time.Time, timezoneOffsetHours float64) map[string]string {
	// Julian Date calculation
	y := float64(targetDate.Year())
	m := float64(targetDate.Month())
	d := float64(targetDate.Day())

	if m <= 2 {
		y -= 1
		m += 12
	}
	A := math.Floor(y / 100)
	B := 2 - A + math.Floor(A/4)
	JD := math.Floor(365.25*(y+4716)) + math.Floor(30.6001*(m+1)) + d + B - 1524.5

	// Days since J2000.0
	D := JD - 2451545.0

	// Sun coordinates
	rad := math.Pi / 180.0
	deg := 180.0 / math.Pi

	g := math.Mod(357.529+0.98560028*D, 360) * rad
	q := math.Mod(280.459+0.98564736*D, 360) * rad
	L := math.Mod(q*deg+1.915*math.Sin(g)+0.020*math.Sin(2*g), 360) * rad

	e := (23.439 - 0.00000036*D) * rad

	// Declination
	delta := math.Asin(math.Sin(e) * math.Sin(L))
	// Equation of Time
	RA := math.Atan2(math.Cos(e)*math.Sin(L), math.Cos(L)) / 15.0
	if RA < 0 {
		RA += 24.0
	}
	EqT := (q*deg)/15.0 - RA
	for EqT > 12 {
		EqT -= 24
	}
	for EqT < -12 {
		EqT += 24
	}

	latRad := lat * rad

	// Solar Noon (Transit)
	transit := 12.0 + timezoneOffsetHours - (lng / 15.0) - EqT

	// Hour angle for a given sun angle
	calcHourAngle := func(angleDeg float64) float64 {
		altRad := angleDeg * rad
		cosHA := (math.Sin(altRad) - math.Sin(latRad)*math.Sin(delta)) / (math.Cos(latRad) * math.Cos(delta))
		if cosHA < -1.0 || cosHA > 1.0 {
			return 0
		}
		return math.Acos(cosHA) * deg / 15.0
	}

	// Subuh: Fajr angle = -20°
	haFajr := calcHourAngle(-20.0)
	fajr := transit - haFajr

	// Dzuhur: Transit + 2-3 mins (center of sun crosses meridian)
	dzuhur := transit + (2.0 / 60.0)

	// Ashar: Shafi'i (shadow length = object length + shadow at noon)
	noonAlt := (math.Pi / 2.0) - math.Abs(latRad-delta)
	asharAngle := math.Atan(1.0/(1.0+math.Tan(math.Abs(latRad-delta)))) * deg
	if noonAlt > 0 {
		asharAngle = math.Atan(1.0/(1.0+1.0/math.Tan(noonAlt))) * deg
	}
	haAshar := calcHourAngle(asharAngle)
	ashar := transit + haAshar

	// Maghrib: Sun set angle = -0.833°
	haSunset := calcHourAngle(-0.833)
	maghrib := transit + haSunset

	// Isya: angle = -18°
	haIsha := calcHourAngle(-18.0)
	isya := transit + haIsha

	formatHour := func(hours float64) string {
		for hours < 0 {
			hours += 24
		}
		for hours >= 24 {
			hours -= 24
		}
		totalSecs := int(math.Round(hours * 3600))
		h := (totalSecs / 3600) % 24
		m := (totalSecs % 3600) / 60
		return fmt.Sprintf("%02d:%02d", h, m)
	}

	return map[string]string{
		"subuh":   formatHour(fajr),
		"dzuhur":  formatHour(dzuhur),
		"ashar":   formatHour(ashar),
		"maghrib": formatHour(maghrib),
		"isya":    formatHour(isya),
	}
}

func (r *PrayerResolver) ApplyIhtiyat(timeStr string, ihtiyatMinutes int) string {
	if len(timeStr) != 5 || timeStr[2] != ':' {
		return timeStr
	}
	h, _ := strconv.Atoi(timeStr[:2])
	m, _ := strconv.Atoi(timeStr[3:])

	totalMinutes := h*60 + m + ihtiyatMinutes
	for totalMinutes < 0 {
		totalMinutes += 1440
	}
	totalMinutes = totalMinutes % 1440

	return fmt.Sprintf("%02d:%02d", totalMinutes/60, totalMinutes%60)
}

func (r *PrayerResolver) CalculatePreAlert(timeStr string, offsetMinutes int) string {
	return r.ApplyIhtiyat(timeStr, -offsetMinutes)
}

func (r *PrayerResolver) GetTodaySchedule(targetDate time.Time, tzLocation *time.Location) PrayerSchedule {
	config, _ := database.GetPrayerConfig()
	enabled, _ := config["enabled"].(bool)
	if !enabled {
		return PrayerSchedule{
			Enabled: false,
			Date:    targetDate.Format("2006-01-02"),
			Times:   map[string]string{},
			Source:  "disabled",
			Config:  config,
		}
	}

	cityID := "1301"
	if s, ok := config["city_id"].(string); ok && s != "" {
		cityID = s
	}

	cityName := "Kota Jakarta"
	if s, ok := config["city_name"].(string); ok && s != "" {
		cityName = s
	}

	lat := -6.2088
	if f, ok := config["latitude"].(float64); ok {
		lat = f
	}

	lng := 106.8456
	if f, ok := config["longitude"].(float64); ok {
		lng = f
	}

	ihtiyat := 2
	if f, ok := config["ihtiyat_minutes"].(float64); ok {
		ihtiyat = int(f)
	}

	reminderOffset := 2
	if f, ok := config["reminder_offset_minutes"].(float64); ok {
		reminderOffset = int(f)
	}

	// 1. Try API first
	rawTimes, err := r.FetchFromAPI(cityID, targetDate)
	source := "api"
	if err != nil || rawTimes == nil {
		// Fallback to offline calculation
		_, offsetSecs := targetDate.In(tzLocation).Zone()
		tzHours := float64(offsetSecs) / 3600.0
		rawTimes = r.CalculateOffline(lat, lng, targetDate, tzHours)
		source = "offline_calculated"
	}

	// Apply Ihtiyat
	finalTimes := make(map[string]string)
	for p, t := range rawTimes {
		finalTimes[p] = r.ApplyIhtiyat(t, ihtiyat)
	}

	// Calculate Reminder times (e.g. T-2 minutes before prayer)
	reminders := make(map[string]string)
	for _, p := range []string{"dzuhur", "ashar", "maghrib"} {
		if t, ok := finalTimes[p]; ok {
			reminders[p] = r.CalculatePreAlert(t, reminderOffset)
		}
	}

	msg := fmt.Sprintf("Synced prayer reminders (%s): Dzuhur reminder=%s, Ashar reminder=%s, Maghrib reminder=%s",
		source, reminders["dzuhur"], reminders["ashar"], reminders["maghrib"])
	database.LogActivity("prayer_sync", "prayer_resolver", "", "", "success", msg)

	return PrayerSchedule{
		Enabled:   true,
		Date:      targetDate.Format("2006-01-02"),
		Source:    source,
		CityName:  cityName,
		Times:     finalTimes,
		Reminders: reminders,
		PreAlerts: reminders,
		Config:    config,
	}
}
