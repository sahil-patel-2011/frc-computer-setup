//go:build !windows

package host

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command("xdg-open", url).Start()
		}
		fmt.Printf("Open this page: %s\n", url)
		return nil
	}
}

func installWPILibISO(isoPath string) error {
	return fmt.Errorf("WPILib ISO install is Windows-only (got %s). File is at %s", runtime.GOOS, isoPath)
}
