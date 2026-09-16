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

func installWPILibISO(isoPath string, args []string) error {
	if !HasSilentFlags(args) {
		return fmt.Errorf("needs vendor page: WPILib has no silent flags")
	}
	argList := powershellArgList(args)
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$iso = '%s'
$img = Mount-DiskImage -ImagePath $iso -PassThru
try {
  $vol = ($img | Get-Volume)
  $letter = $vol.DriveLetter
  if (-not $letter) { throw 'ISO mounted but no drive letter' }
  $root = $letter + ':'
  $cli = $null
  foreach ($name in @('WPILibInstallerCLI.exe','WPILibInstaller.CLI.exe','WPILibInstaller-CLI.exe')) {
    $p = Join-Path $root $name
    if (Test-Path $p) { $cli = $p; break }
  }
  if (-not $cli) { throw 'needs vendor page: this WPILib ISO has no silent CLI installer' }
  $p = Start-Process -FilePath $cli -ArgumentList %s -Wait -PassThru
  if ($p.ExitCode -ne 0) { throw "WPILib silent installer exit $($p.ExitCode)" }
} finally {
  Dismount-DiskImage -ImagePath $iso | Out-Null
}
`, strings.ReplaceAll(isoPath, "'", "''"), argList)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func powershellArgList(args []string) string {
	if len(args) == 0 {
		return "@()"
	}
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, "'"+strings.ReplaceAll(a, "'", "''")+"'")
	}
	return "@(" + strings.Join(parts, ",") + ")"
}

func installDMG(path string) error {
	return fmt.Errorf("DMG install is macOS-only")
}

func installWPILibDMG(path string, args []string) error {
	return fmt.Errorf("WPILib DMG install is macOS-only (got %s)", path)
}

func installTarball(path string) error {
	return fmt.Errorf("tarball install is not used on Windows (got %s)", path)
}

func installWPILibTarball(path string, args []string) error {
	return installTarball(path)
}

func installZip(path string) error {
	return fmt.Errorf("zip install is not used on Windows (got %s)", path)
}

func installAppImage(path string) error {
	return fmt.Errorf("AppImage install is Linux-only")
}
