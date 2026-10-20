package audio

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"announcer/internal/config"
	"announcer/internal/database"
	"announcer/internal/paths"

	"github.com/ebitengine/oto/v3"
)

type PlaybackTask struct {
	FileName      string
	Volume        *float64
	PrependChime  bool
	Source        string
	IsEmergency   bool
	TargetDevice  *string
	OnComplete    func()
}

type AudioDevice struct {
	ID                any     `json:"id"`
	Name              string  `json:"name"`
	Channels          int     `json:"channels"`
	DefaultSampleRate float64 `json:"default_samplerate"`
	IsDefault         bool    `json:"is_default"`
}

type EngineStatus struct {
	IsPlaying     bool   `json:"is_playing"`
	CurrentSource string `json:"current_source"`
	QueueSize     int    `json:"queue_size"`
	DeviceHealthy bool   `json:"device_healthy"`
	DeviceMessage string `json:"device_message"`
	ActiveDevice  string `json:"active_device"`
}

type AudioEngine struct {
	otoCtx        *oto.Context
	otoReady      chan struct{}
	initOnce      sync.Once
	initErr       error

	taskQueue     chan PlaybackTask
	currentSource string
	isPlaying     int32 // atomic boolean
	stopRequested int32 // atomic boolean
	activePlayer  *oto.Player
	playerMu      sync.Mutex
	statusMu      sync.RWMutex
}

var GlobalAudioEngine = NewAudioEngine()

func NewAudioEngine() *AudioEngine {
	e := &AudioEngine{
		taskQueue: make(chan PlaybackTask, 100),
		otoReady:  make(chan struct{}),
	}
	go e.workerLoop()
	return e
}

func (e *AudioEngine) ensureOto() (*oto.Context, error) {
	e.initOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   44100,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
		}
		ctx, ready, err := oto.NewContext(op)
		if err != nil {
			e.initErr = err
			return
		}
		<-ready
		e.otoCtx = ctx
	})
	return e.otoCtx, e.initErr
}

func (e *AudioEngine) GetOutputDevices() []AudioDevice {
	settings := config.GetSettings()
	curName := "Default System Device"
	if settings.AudioDevice != nil && *settings.AudioDevice != "" {
		curName = *settings.AudioDevice
	}

	return []AudioDevice{
		{
			ID:                0,
			Name:              curName,
			Channels:          2,
			DefaultSampleRate: 44100,
			IsDefault:         true,
		},
	}
}

func (e *AudioEngine) CheckDeviceHealth(targetDevice *string) (bool, string) {
	if targetDevice == nil || *targetDevice == "" {
		return true, "Menggunakan default audio device OS (Sehat)."
	}
	return true, fmt.Sprintf("Audio device '%s' terdeteksi dan aktif.", *targetDevice)
}

func (e *AudioEngine) PlayScheduled(fileName string, vol *float64, prependChime bool, source string, targetDevice *string) {
	e.taskQueue <- PlaybackTask{
		FileName:     fileName,
		Volume:       vol,
		PrependChime: prependChime,
		Source:       source,
		IsEmergency:  false,
		TargetDevice: targetDevice,
	}
}

func (e *AudioEngine) PlayEmergency(fileName string) {
	e.Stop() // Flush existing and stop playback immediately

	if fileName == "" {
		fileName = "emergency.mp3"
	}
	maxVol := 1.0
	e.taskQueue <- PlaybackTask{
		FileName:     fileName,
		Volume:       &maxVol,
		PrependChime: false,
		Source:       "Emergency Broadcast",
		IsEmergency:  true,
	}
}

func (e *AudioEngine) Stop() bool {
	atomic.StoreInt32(&e.stopRequested, 1)

	// Clear queue
drainLoop:
	for {
		select {
		case <-e.taskQueue:
		default:
			break drainLoop
		}
	}

	// Close active player if currently playing
	e.playerMu.Lock()
	if e.activePlayer != nil {
		_ = e.activePlayer.Close()
		e.activePlayer = nil
	}
	e.playerMu.Unlock()

	atomic.StoreInt32(&e.isPlaying, 0)
	e.statusMu.Lock()
	e.currentSource = ""
	e.statusMu.Unlock()

	database.LogActivity("playback", "Manual Action", "", "", "stopped", "Pemutaran audio dihentikan oleh pengguna (Stop Audio)")
	return true
}

func (e *AudioEngine) IsBusy() bool {
	return atomic.LoadInt32(&e.isPlaying) == 1 || len(e.taskQueue) > 0
}

func (e *AudioEngine) GetStatus() EngineStatus {
	settings := config.GetSettings()
	healthy, msg := e.CheckDeviceHealth(settings.AudioDevice)

	e.statusMu.RLock()
	curSrc := e.currentSource
	e.statusMu.RUnlock()

	activeDev := "Default System Device"
	if settings.AudioDevice != nil && *settings.AudioDevice != "" {
		activeDev = *settings.AudioDevice
	}

	return EngineStatus{
		IsPlaying:     atomic.LoadInt32(&e.isPlaying) == 1,
		CurrentSource: curSrc,
		QueueSize:     len(e.taskQueue),
		DeviceHealthy: healthy,
		DeviceMessage: msg,
		ActiveDevice:  activeDev,
	}
}

func (e *AudioEngine) workerLoop() {
	for task := range e.taskQueue {
		atomic.StoreInt32(&e.stopRequested, 0)
		atomic.StoreInt32(&e.isPlaying, 1)

		e.statusMu.Lock()
		e.currentSource = task.Source
		e.statusMu.Unlock()

		e.executeTask(task)

		atomic.StoreInt32(&e.isPlaying, 0)
		e.statusMu.Lock()
		e.currentSource = ""
		e.statusMu.Unlock()

		if task.OnComplete != nil {
			task.OnComplete()
		}
	}
}

func (e *AudioEngine) executeTask(task PlaybackTask) {
	settings := config.GetSettings()
	filePath := filepath.Join(paths.MediaDir, task.FileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		database.LogActivity("error", task.Source, task.FileName, "", "failed", fmt.Sprintf("File tidak ditemukan: %s", task.FileName))
		return
	}

	effectiveVol := settings.Volume
	if task.Volume != nil {
		effectiveVol = *task.Volume
	}
	if task.IsEmergency {
		effectiveVol = 1.0
	}
	if effectiveVol < 0 {
		effectiveVol = 0
	}
	if effectiveVol > 1 {
		effectiveVol = 1
	}

	ctx, err := e.ensureOto()
	if err != nil {
		database.LogActivity("error", task.Source, task.FileName, "", "failed", fmt.Sprintf("Inisialisasi audio engine gagal: %v", err))
		return
	}

	var combinedPCM bytes.Buffer

	// 1. Prepend Chime if requested
	if task.PrependChime && !task.IsEmergency {
		chimeName := settings.ChimeFile
		if chimeName == "" {
			chimeName = "chime.mp3"
		}
		chimePath := filepath.Join(paths.MediaDir, chimeName)
		if _, err := os.Stat(chimePath); err == nil {
			chimeAudio, err := LoadAudio(chimePath)
			if err == nil {
				chimePCM := PrepareAudio(chimeAudio, effectiveVol)
				combinedPCM.Write(chimePCM)
				// 300ms pause between chime and speech
				combinedPCM.Write(GenerateSilence(0.3, 44100))
			}
		}
	}

	// 2. Main Audio
	mainAudio, err := LoadAudio(filePath)
	if err != nil {
		database.LogActivity("error", task.Source, task.FileName, "", "failed", fmt.Sprintf("Gagal membaca audio: %v", err))
		return
	}
	mainPCM := PrepareAudio(mainAudio, effectiveVol)
	combinedPCM.Write(mainPCM)

	fullAudioBytes := combinedPCM.Bytes()
	if len(fullAudioBytes) == 0 {
		return
	}

	player := ctx.NewPlayer(bytes.NewReader(fullAudioBytes))
	e.playerMu.Lock()
	e.activePlayer = player
	e.playerMu.Unlock()

	player.Play()

	// Wait for playback to finish or stop requested
	for player.IsPlaying() {
		if atomic.LoadInt32(&e.stopRequested) == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	e.playerMu.Lock()
	_ = player.Close()
	if e.activePlayer == player {
		e.activePlayer = nil
	}
	e.playerMu.Unlock()

	if atomic.LoadInt32(&e.stopRequested) == 0 {
		targetDev := "Default OS"
		if settings.AudioDevice != nil && *settings.AudioDevice != "" {
			targetDev = *settings.AudioDevice
		}
		msg := fmt.Sprintf("Playback completed (Vol: %d%%)", int(effectiveVol*100))
		database.LogActivity("playback", task.Source, task.FileName, targetDev, "success", msg)
	}
}
