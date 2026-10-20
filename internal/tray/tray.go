package tray

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

type TrayManager struct {
	port   int
	onExit func()
}

func StartTray(port int, onExit func()) {
	if runtime.GOOS == "darwin" {
		// macOS Cocoa requires NSApplication to run on main thread; skip on dev Mac CLI
		return
	}

	m := &TrayManager{
		port:   port,
		onExit: onExit,
	}

	go systray.Run(m.onReady, m.onExitClean)
}

func (m *TrayManager) onReady() {
	if runtime.GOOS == "windows" {
		systray.SetIcon(GenerateIconICO())
	} else {
		systray.SetIcon(GenerateIconPNG())
	}

	systray.SetTitle("Smart Office Announcer")
	systray.SetTooltip("Smart Office Announcement & Prayer Alert System")

	mOpen := systray.AddMenuItem("Buka Dashboard", "Buka Dashboard di Web Browser")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Keluar", "Tutup Aplikasi")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				url := fmt.Sprintf("http://127.0.0.1:%d", m.port)
				OpenBrowser(url)
			case <-mQuit.ClickedCh:
				systray.Quit()
				if m.onExit != nil {
					m.onExit()
				}
				return
			}
		}
	}()
}

func (m *TrayManager) onExitClean() {
	if m.onExit != nil {
		m.onExit()
	}
}

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
