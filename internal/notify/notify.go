package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ShowNotification(title, message string) {
	switch runtime.GOOS {
	case "windows":
		showWindowsToast(title, message)
	case "darwin":
		showMacNotification(title, message)
	}
}

func showMacNotification(title, message string) {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	_ = cmd.Run()
}

func showWindowsToast(title, message string) {
	// Clean quotes to prevent script injection
	cleanTitle := strings.ReplaceAll(title, `"`, `\"`)
	cleanMessage := strings.ReplaceAll(message, `"`, `\"`)

	psScript := fmt.Sprintf(`
		[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
		$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
		$textNodes = $template.GetElementsByTagName("text")
		$textNodes.Item(0).AppendChild($template.CreateTextNode("%s")) > $null
		$textNodes.Item(1).AppendChild($template.CreateTextNode("%s")) > $null
		$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
		[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Smart Office Announcer").Show($toast)
	`, cleanTitle, cleanMessage)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	_ = cmd.Start()
}
