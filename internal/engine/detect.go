package engine

import (
	"strconv"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

type Detection struct {
	Installed bool
	Version   string
	Current   bool
}

func DetectTool(h host.Host, tool manifest.Tool) Detection {
	if h == nil || tool.Verify == nil {
		return Detection{}
	}
	if VerifyTool(h, tool) != nil {
		return Detection{}
	}
	d := Detection{Installed: true}
	d.Version = readVersion(h, tool)
	if tool.Pinned != nil {
		d.Current = VersionCurrent(d.Version, tool.Pinned.Version)
	}
	return d
}

func readVersion(h host.Host, tool manifest.Tool) string {
	for _, c := range tool.Verify.Commands {
		name, err := h.LookPath(c.Name)
		if err != nil {
			continue
		}
		out, err := h.Run(name, c.Args...)
		if err != nil {
			continue
		}
		if v := NormalizeVersion(out); v != "" && versionToken.MatchString(v) {
			return v
		}
	}
	return ""
}

func WPILibVSCodeInstalled(h host.Host, season int) bool {
	if h == nil {
		return false
	}
	y := "2026"
	if season > 0 {
		y = strconv.Itoa(season)
	}
	paths := []string{
		`%PUBLIC%\wpilib\` + y + `\vscode\Code.exe`,
		`%USERPROFILE%\wpilib\` + y + `\vscode\Code.exe`,
		`C:\Users\Public\wpilib\` + y + `\vscode\Code.exe`,
		`$HOME/wpilib/` + y + `/vscode/code`,
		`$HOME/wpilib/` + y + `/vscode/Code.exe`,
	}
	return host.Verify(h, paths, nil) == nil
}
