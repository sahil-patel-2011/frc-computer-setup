//go:build windows

package host

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func openURL(url string) error {
	return exec.Command("cmd", "/c", "start", "", url).Start()
}

func installWPILibISO(isoPath string) error {
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$iso = '%s'
$img = Mount-DiskImage -ImagePath $iso -PassThru
try {
  $vol = ($img | Get-Volume)
  $letter = $vol.DriveLetter
  if (-not $letter) { throw 'ISO mounted but no drive letter' }
  $setup = Join-Path ($letter + ':') 'WPILibInstaller.exe'
  if (-not (Test-Path $setup)) { throw "WPILibInstaller.exe not found on $letter" }
  $p = Start-Process -FilePath $setup -Wait -PassThru
  if ($p.ExitCode -ne 0) { throw "WPILib installer exit $($p.ExitCode)" }
} finally {
  Dismount-DiskImage -ImagePath $iso | Out-Null
}
`, strings.ReplaceAll(isoPath, "'", "''"))
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installDMG(path string) error {
	return fmt.Errorf("DMG install is macOS-only")
}

func installTarball(path string) error {
	return fmt.Errorf("tarball install is not used on Windows (got %s)", path)
}

func installWPILibTarball(path string) error {
	return installTarball(path)
}

func installZip(path string) error {
	return fmt.Errorf("zip install is not used on Windows (got %s)", path)
}

func installAppImage(path string) error {
	return fmt.Errorf("AppImage install is Linux-only")
}
