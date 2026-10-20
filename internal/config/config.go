package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"announcer/internal/paths"
)

type SystemSettings struct {
	Port            int      `json:"port"`
	Timezone        string   `json:"timezone"`
	Volume          float64  `json:"volume"`
	PrependChime    bool     `json:"prepend_chime"`
	ChimeFile       string   `json:"chime_file"`
	AudioDevice     *string  `json:"audio_device"`
	AutoOpenBrowser bool     `json:"auto_open_browser"`
	AutoStartOnBoot bool     `json:"auto_start_on_boot"`
}

type AppConfig struct {
	SystemSettings SystemSettings `json:"system_settings"`
}

var (
	configMu sync.RWMutex
	cachedConfig AppConfig
)

func DefaultConfig() AppConfig {
	return AppConfig{
		SystemSettings: SystemSettings{
			Port:            5050,
			Timezone:        "Asia/Jakarta",
			Volume:          0.85,
			PrependChime:    true,
			ChimeFile:       "chime.mp3",
			AudioDevice:     nil,
			AutoOpenBrowser: true,
			AutoStartOnBoot: true,
		},
	}
}

func LoadConfig() AppConfig {
	configMu.Lock()
	defer configMu.Unlock()

	cfgFile := paths.ConfigFile
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		def := DefaultConfig()
		cachedConfig = def
		saveConfigLocked(def)
		return def
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		def := DefaultConfig()
		cachedConfig = def
		return def
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		def := DefaultConfig()
		cachedConfig = def
		return def
	}

	// Fallback empty fields to defaults
	if cfg.SystemSettings.Port == 0 {
		cfg.SystemSettings.Port = 5050
	}
	if cfg.SystemSettings.Timezone == "" {
		cfg.SystemSettings.Timezone = "Asia/Jakarta"
	}
	if cfg.SystemSettings.ChimeFile == "" {
		cfg.SystemSettings.ChimeFile = "chime.mp3"
	}

	cachedConfig = cfg
	return cfg
}

func GetSettings() SystemSettings {
	configMu.RLock()
	defer configMu.RUnlock()
	if cachedConfig.SystemSettings.Port == 0 {
		configMu.RUnlock()
		cfg := LoadConfig()
		configMu.RLock()
		return cfg.SystemSettings
	}
	return cachedConfig.SystemSettings
}

func UpdateSettings(rawUpdates map[string]any) (SystemSettings, error) {
	configMu.Lock()
	defer configMu.Unlock()

	cur := cachedConfig
	if cur.SystemSettings.Port == 0 {
		cur = DefaultConfig()
	}

	// Marshal and unmarshal partial json into current struct
	curBytes, _ := json.Marshal(cur.SystemSettings)
	var curMap map[string]any
	_ = json.Unmarshal(curBytes, &curMap)

	for k, v := range rawUpdates {
		curMap[k] = v
	}

	mergedBytes, err := json.Marshal(curMap)
	if err != nil {
		return cur.SystemSettings, err
	}

	var updatedSettings SystemSettings
	if err := json.Unmarshal(mergedBytes, &updatedSettings); err != nil {
		return cur.SystemSettings, err
	}

	cur.SystemSettings = updatedSettings
	if err := saveConfigLocked(cur); err != nil {
		return cur.SystemSettings, err
	}

	cachedConfig = cur
	return cur.SystemSettings, nil
}

func saveConfigLocked(cfg AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	cfgFile := paths.ConfigFile
	tmpFile := filepath.Join(filepath.Dir(cfgFile), "config.tmp.json")
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, cfgFile)
}
