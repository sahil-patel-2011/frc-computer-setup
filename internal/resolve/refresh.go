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
		pin, err := PinFromGitHub(rel, tool.Source.Asset, now)
		if err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		if err := c.FillHeaders(&pin); err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		return Result{Pin: pin}
	case tool.Source.Type == "wpilib_github_notes":
		rel, err := c.Latest(tool.Source.Owner, tool.Source.Repo)
		if err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		pin, err := PinFromWPILibNotes(rel, now)
		if err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		if err := c.FillHeaders(&pin); err != nil {
			return Result{VendorFallback: true, Reason: err.Error()}
		}
		return Result{Pin: pin}
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
	tool.UnprovenReason = ""
}
