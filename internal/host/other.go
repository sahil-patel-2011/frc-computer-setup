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

func installWPILibISO(isoPath string, args []string) error {
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
		return fmt.Errorf("needs vendor page: this DMG has no app to copy silently")
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
	_, err := extractTarGz(path)
	return err
}

func installWPILibDMG(path string, args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("WPILib DMG install is macOS-only")
	}
	if !HasSilentFlags(args) {
		return fmt.Errorf("needs vendor page: WPILib has no silent flags")
	}
	mount := filepath.Join(os.TempDir(), fmt.Sprintf("frc-wpilib-dmg-%d", os.Getpid()))
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
	cli := findWPILibCLI(mount)
	if cli == "" {
		return fmt.Errorf("needs vendor page: this WPILib DMG has no silent CLI installer")
	}
	if err := os.Chmod(cli, 0o755); err != nil {
		return err
	}
	cmd := exec.Command(cli, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installWPILibTarball(path string, args []string) error {
	if !HasSilentFlags(args) {
		return fmt.Errorf("needs vendor page: WPILib has no silent flags")
	}
	dir, err := extractTarGz(path)
	if err != nil {
		return err
	}
	cli := findWPILibCLI(dir)
	if cli == "" {
		return fmt.Errorf("needs vendor page: this WPILib archive has no silent CLI installer")
	}
	if err := os.Chmod(cli, 0o755); err != nil {
		return err
	}
	cmd := exec.Command(cli, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findWPILibCLI(root string) string {
	names := []string{
		"WPILibInstallerCLI",
		"WPILibInstaller.CLI",
		"WPILibInstaller-CLI",
	}
	var found string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		for _, n := range names {
			if base == n {
				found = p
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

func extractTarGz(path string) (string, error) {
	dir := strings.TrimSuffix(path, ".tar.gz")
	if dir == path {
		dir = path + ".dir"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("tar", "-xf", path, "-C", dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return dir, nil
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
