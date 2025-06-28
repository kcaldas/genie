package helpers

import (
	"fmt"
	"os/exec"
	"runtime"
)

type NotificationHelper struct{}

func NewNotificationHelper() *NotificationHelper {
	return &NotificationHelper{}
}

func (h *NotificationHelper) Send(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return h.sendMacNotification(title, message)
	case "linux":
		return h.sendLinuxNotification(title, message)
	case "windows":
		return h.sendWindowsNotification(title, message)
	default:
		return fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}
}

func (h *NotificationHelper) sendMacNotification(title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

func (h *NotificationHelper) sendLinuxNotification(title, message string) error {
	cmd := exec.Command("notify-send", title, message)
	return cmd.Run()
}

func (h *NotificationHelper) sendWindowsNotification(title, message string) error {
	script := fmt.Sprintf(`
		Add-Type -AssemblyName System.Windows.Forms
		$notification = New-Object System.Windows.Forms.NotifyIcon
		$notification.Icon = [System.Drawing.SystemIcons]::Information
		$notification.BalloonTipIcon = 'Info'
		$notification.BalloonTipTitle = '%s'
		$notification.BalloonTipText = '%s'
		$notification.Visible = $true
		$notification.ShowBalloonTip(5000)
	`, title, message)
	
	cmd := exec.Command("powershell", "-Command", script)
	return cmd.Run()
}

func (h *NotificationHelper) IsAvailable() bool {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("which", "osascript")
		return cmd.Run() == nil
	case "linux":
		cmd := exec.Command("which", "notify-send")
		return cmd.Run() == nil
	case "windows":
		return true
	default:
		return false
	}
}