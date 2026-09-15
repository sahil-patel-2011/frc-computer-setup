package resolve

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

var (
	winISOLink = regexp.MustCompile(`https://packages\.wpilib\.workers\.dev/installer/(v[^/]+)/Win64/(WPILib_Windows-[^)\s]+\.iso)`)
	winSHALine = regexp.MustCompile(`(?i)([0-9a-f]{64})\s+Win64/(WPILib_Windows-\S+\.iso)`)
)

func PinFromWPILibNotes(rel *GitHubRelease, now time.Time) (manifest.Pin, error) {
	if rel == nil {
		return manifest.Pin{}, fmt.Errorf("missing WPILib release")
	}
	m := winISOLink.FindStringSubmatch(rel.Body)
	if m == nil {
		return manifest.Pin{}, fmt.Errorf("WPILib %s notes have no Windows ISO URL — vendor page, do not invent", rel.TagName)
	}
	url := m[0]
	file := m[2]
	sha := ""
	if sm := winSHALine.FindStringSubmatch(rel.Body); sm != nil && sm[2] == file {
		sha = strings.ToLower(sm[1])
	}
	if sha == "" {
		if got, ok := SHA256FromBody(rel.Body, file); ok {
			sha = got
		}
	}
	if sha == "" {
		return manifest.Pin{}, fmt.Errorf("WPILib %s notes have Windows ISO but no SHA-256", rel.TagName)
	}
	return manifest.Pin{
		Version:    rel.TagName,
		URL:        url,
		SHA256:     sha,
		AssetName:  file,
		ReleaseURL: rel.HTMLURL,
		ProvenAt:   now.UTC().Format(time.RFC3339),
	}, nil
}
