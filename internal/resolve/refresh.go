package resolve

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func (c *Client) RefreshTool(tool manifest.Tool) Result {
	now := c.Now()
	switch {
	case tool.Kind == "vendor_page":
		return Result{VendorFallback: true, Reason: tool.UnprovenReason}
	case tool.Source == nil:
		return Result{VendorFallback: true, Reason: "no source; keeping vendor step rather than inventing a URL"}
	case tool.Source.Type == "github_release":
		rel, err := c.Latest(tool.Source.Owner, tool.Source.Repo)
		if err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		assets := map[string]string{}
		for k, v := range tool.Source.Assets {
			assets[k] = v
		}
		if len(assets) == 0 && tool.Source.Asset != "" {
			assets["windows-amd64"] = tool.Source.Asset
		}
		if len(assets) == 0 {
			return Result{VendorFallback: true, Reason: "no asset glob; not inventing a download"}
		}
		pins := map[string]manifest.Pin{}
		var first manifest.Pin
		for key, glob := range assets {
			pin, err := PinFromGitHub(rel, glob, now)
			if err != nil {
				continue
			}
			if err := c.FillHeaders(&pin); err != nil {
				continue
			}
			pins[key] = pin
			if first.URL == "" || key == "windows-amd64" {
				first = pin
			}
		}
		if len(pins) == 0 {
			return Result{VendorFallback: true, Reason: "no proven GitHub assets for this release"}
		}
		return Result{Pin: first, Pins: pins}
	case tool.Source.Type == "wpilib_github_notes":
		rel, err := c.Latest(tool.Source.Owner, tool.Source.Repo)
		if err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		pins, err := PinsFromWPILibNotes(rel, now)
		if err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		for key, pin := range pins {
			p := pin
			if err := c.FillHeaders(&p); err != nil {
				delete(pins, key)
				continue
			}
			pins[key] = p
		}
		if len(pins) == 0 {
			return Result{VendorFallback: true, Reason: "WPILib notes URLs did not HEAD successfully"}
		}
		first := pins["windows-amd64"]
		if first.URL == "" {
			for _, p := range pins {
				first = p
				break
			}
		}
		return Result{Pin: first, Pins: pins}
	default:
		return Result{VendorFallback: true, Reason: fmt.Sprintf("unknown source type %q", tool.Source.Type)}
	}
}

func (c *Client) FillHeaders(pin *manifest.Pin) error {
	req, err := http.NewRequest(http.MethodHead, pin.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("HEAD %s: HTTP %d", pin.URL, res.StatusCode)
	}
	if etag := strings.TrimSpace(res.Header.Get("ETag")); etag != "" {
		pin.ETag = etag
	}
	if res.ContentLength > 0 {
		pin.Size = res.ContentLength
	}
	return nil
}

func Apply(tool *manifest.Tool, result Result) {
	if result.VendorFallback {
		tool.Kind = "vendor_page"
		tool.Pinned = nil
		tool.Pins = nil
		if tool.UnprovenReason == "" {
			tool.UnprovenReason = result.Reason
		}
		if tool.VendorURL == "" && tool.DocsURL != "" {
			tool.VendorURL = tool.DocsURL
		}
		return
	}
	tool.Kind = "download"
	pin := result.Pin
	tool.Pinned = &pin
	if result.Pins != nil {
		tool.Pins = result.Pins
		if p, ok := result.Pins["windows-amd64"]; ok {
			cp := p
			tool.Pinned = &cp
		}
	}
	tool.UnprovenReason = ""
}
