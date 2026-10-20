package paths

import (
	"os"
	"path/filepath"
	"strings"
)

var (
	AppDir     string
	MediaDir   string
	LogsDir    string
	DBFile     string
	ConfigFile string
)

func init() {
	AppDir = resolveAppDir()
	MediaDir = filepath.Join(AppDir, "media")
	LogsDir = filepath.Join(AppDir, "logs")
	DBFile = filepath.Join(AppDir, "data.db")
	ConfigFile = filepath.Join(AppDir, "config.json")
}

func resolveAppDir() string {
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		// If running via `go run`, executable is placed in a temp folder
		if !strings.Contains(exeDir, "go-build") && !strings.Contains(exeDir, "var/folders") && !strings.Contains(exeDir, "AppData\\Local\\Temp") {
			return exeDir
		}
	}
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	return "."
}

func InitDirectories() error {
	for _, dir := range []string{MediaDir, LogsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}
