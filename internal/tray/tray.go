package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	mOpen := systray.AddMenuItem("Buka Dashboard", "Buka Aplikasi Desktop")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Keluar", "Tutup Aplikasi Sepenuhnya")

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

// OpenBrowser launches the dashboard. On Windows, it prefers Standalone App Window mode
// (without browser address bar or tabs), feeling like a native desktop app.
func OpenBrowser(url string) {
	if runtime.GOOS == "windows" {
		if openWindowsAppWindow(url) {
			return
		}
		// Fallback to default browser if app mode is not possible
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		_ = cmd.Start()
		return
	}

	if runtime.GOOS == "darwin" {
		cmd := exec.Command("open", url)
		_ = cmd.Start()
		return
	}

	cmd := exec.Command("xdg-open", url)
	_ = cmd.Start()
}

func openWindowsAppWindow(url string) bool {
	// Candidate Edge & Chrome browser paths on Windows
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p, fmt.Sprintf("--app=%s", url), "--window-size=1200,800")
			if err := cmd.Start(); err == nil {
				return true
			}
		}
	}

	// Try via PATH lookup
	for _, bin := range []string{"msedge", "chrome"} {
		if lp, err := exec.LookPath(bin); err == nil {
			cmd := exec.Command(lp, fmt.Sprintf("--app=%s", url), "--window-size=1200,800")
			if err := cmd.Start(); err == nil {
				return true
			}
		}
	}

	// Try via Windows start command
	cmd := exec.Command("cmd", "/c", "start", "", "msedge", fmt.Sprintf("--app=%s", url), "--window-size=1200,800")
	if err := cmd.Start(); err == nil {
		return true
	}

	return false
}
