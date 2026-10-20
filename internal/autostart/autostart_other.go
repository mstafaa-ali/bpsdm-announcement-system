//go:build !windows

package autostart

func SetAutoStart(enable bool) bool {
	// Gracefully no-op on non-Windows platforms
	return true
}
