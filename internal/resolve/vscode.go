package resolve

import (
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

var vscodeVer = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func (c *Client) pinVSCodeLatest(channel string, now time.Time) (manifest.Pin, error) {
	latest := "https://update.code.visualstudio.com/latest/" + channel + "/stable"
	req, err := http.NewRequest(http.MethodHead, latest, nil)
	if err != nil {
		return manifest.Pin{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	noFollow := *httpClient
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	res, err := noFollow.Do(req)
	if err != nil {
		return manifest.Pin{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusFound && res.StatusCode != http.StatusMovedPermanently && res.StatusCode != http.StatusTemporaryRedirect && res.StatusCode != http.StatusOK {
		return manifest.Pin{}, fmt.Errorf("vscode latest %s: HTTP %d — not inventing a version", channel, res.StatusCode)
	}
	sha := strings.ToLower(strings.TrimSpace(res.Header.Get("X-SHA256")))
	loc := strings.TrimSpace(res.Header.Get("Location"))
	if sha == "" || !looksSHA256(sha) {
		return manifest.Pin{}, fmt.Errorf("vscode latest %s missing x-sha256 — vendor page, do not invent", channel)
	}
	asset := path.Base(loc)
	if asset == "." || asset == "/" || asset == "" {
		asset = channel
	}
	ver := ""
	if m := vscodeVer.FindStringSubmatch(asset); len(m) > 1 {
		ver = m[1]
	}
	if ver == "" {
		ver = "stable"
	}
	pin := manifest.Pin{
		Version:    ver,
		URL:        latest,
		SHA256:     sha,
		AssetName:  asset,
		ReleaseURL: "https://code.visualstudio.com/Download",
		ProvenAt:   now.UTC().Format(time.RFC3339),
	}
	if loc != "" {
		sizePin := pin
		sizePin.URL = loc
		if err := c.FillHeaders(&sizePin); err == nil && sizePin.Size > 0 {
			pin.Size = sizePin.Size
			if sizePin.ETag != "" {
				pin.ETag = sizePin.ETag
			}
		}
	}
	return pin, nil
}
