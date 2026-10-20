//go:build windows

package autostart

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	regKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	appName = "SmartOfficeAnnouncer"
)

func SetAutoStart(enable bool) bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	if enable {
		val := fmt.Sprintf(`"%s"`, exePath)
		if err := k.SetStringValue(appName, val); err != nil {
			return false
		}
	} else {
		_ = k.DeleteValue(appName)
	}
	return true
}
