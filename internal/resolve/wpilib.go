package resolve

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

var (
	wpilibLink = regexp.MustCompile(`https://packages\.wpilib\.workers\.dev/installer/(v[^/]+)/(Win64|macOSArm|macOS|LinuxArm64|Linux)/(WPILib_[^)\s]+)`)
	wpilibSHA  = regexp.MustCompile(`(?i)([0-9a-f]{64})\s+(Win64|macOSArm|macOS|LinuxArm64|Linux)/(WPILib_\S+)`)
)

var wpilibDirToKey = map[string]string{
	"Win64":      "windows-amd64",
	"macOSArm":   "darwin-arm64",
	"macOS":      "darwin-amd64",
	"Linux":      "linux-amd64",
	"LinuxArm64": "linux-arm64",
}

func PinsFromWPILibNotes(rel *GitHubRelease, now time.Time) (map[string]manifest.Pin, error) {
	if rel == nil {
		return nil, fmt.Errorf("missing WPILib release")
	}
	shas := map[string]string{}
	for _, sm := range wpilibSHA.FindAllStringSubmatch(rel.Body, -1) {
		shas[sm[3]] = strings.ToLower(sm[1])
	}
	pins := map[string]manifest.Pin{}
	proven := now.UTC().Format(time.RFC3339)
	for _, m := range wpilibLink.FindAllStringSubmatch(rel.Body, -1) {
		dir, file := m[2], m[3]
		key, ok := wpilibDirToKey[dir]
		if !ok {
			continue
		}
		sha := shas[file]
		if sha == "" {
			if got, ok := SHA256FromBody(rel.Body, file); ok {
				sha = got
			}
		}
		if sha == "" {
			continue
		}
		pins[key] = manifest.Pin{
			Version:    rel.TagName,
			URL:        m[0],
			SHA256:     sha,
			AssetName:  file,
			ReleaseURL: rel.HTMLURL,
			ProvenAt:   proven,
		}
	}
	if len(pins) == 0 {
		return nil, fmt.Errorf("WPILib %s notes have no proven installer URLs — vendor page, do not invent", rel.TagName)
	}
	return pins, nil
}

func PinFromWPILibNotes(rel *GitHubRelease, now time.Time) (manifest.Pin, error) {
	pins, err := PinsFromWPILibNotes(rel, now)
	if err != nil {
		return manifest.Pin{}, err
	}
	if p, ok := pins["windows-amd64"]; ok {
		return p, nil
	}
	return manifest.Pin{}, fmt.Errorf("WPILib %s notes have no Windows ISO URL — vendor page, do not invent", rel.TagName)
}
