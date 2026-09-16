//go:build !windows

package host

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func installDMG(path string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("DMG install is macOS-only")
	}
	mount := filepath.Join(os.TempDir(), fmt.Sprintf("frc-dmg-%d", os.Getpid()))
	_ = os.RemoveAll(mount)
	if err := os.MkdirAll(mount, 0o755); err != nil {
		return err
	}
	out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mount, path).CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(mount)
		return fmt.Errorf("hdiutil attach: %s: %w", strings.TrimSpace(string(out)), err)
	}
	defer func() {
		_ = exec.Command("hdiutil", "detach", mount, "-quiet").Run()
		_ = os.RemoveAll(mount)
	}()
	apps, _ := filepath.Glob(filepath.Join(mount, "*.app"))
	if len(apps) == 0 {
		apps, _ = filepath.Glob(filepath.Join(mount, "*", "*.app"))
	}
	if len(apps) == 0 {
		// Installer DMG (not a drag-to-Applications app). Vendor GUI.
		return exec.Command("open", "-W", path).Run()
	}
	home, _ := os.UserHomeDir()
	userApps := ""
	if home != "" {
		userApps = filepath.Join(home, "Applications")
	}
	for _, app := range apps {
		if err := copyDir(app, filepath.Join("/Applications", filepath.Base(app))); err != nil {
			if userApps == "" {
				return err
			}
			if mkErr := os.MkdirAll(userApps, 0o755); mkErr != nil {
				return err
			}
			if err := copyDir(app, filepath.Join(userApps, filepath.Base(app))); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src, dest string) error {
	_ = os.RemoveAll(dest)
	cmd := exec.Command("cp", "-R", src, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installTarball(path string) error {
	dir := strings.TrimSuffix(path, ".tar.gz")
	if dir == path {
		dir = path + ".dir"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("tar", "-xf", path, "-C", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	setup := filepath.Join(dir, "WPILibInstaller")
	if _, err := os.Stat(setup); err != nil {
		matches, _ := filepath.Glob(filepath.Join(dir, "*", "WPILibInstaller"))
		if len(matches) == 0 {
			fmt.Printf("Extracted to %s. Run WPILibInstaller from that folder.\n", dir)
			return nil
		}
		setup = matches[0]
	}
	if err := os.Chmod(setup, 0o755); err != nil {
		return err
	}
	return exec.Command(setup).Run()
}

func installZip(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dest := filepath.Join(home, ".local", "share", "frc-computer-setup", strings.TrimSuffix(filepath.Base(path), ".zip"))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return extractZip(path, dest)
}

func installAppImage(path string) error {
	if err := os.Chmod(path, 0o755); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(bin, filepath.Base(path))
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
