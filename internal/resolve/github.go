package resolve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

const userAgent = "frc-computer-setup (https://github.com/sahil-patel-2011/frc-computer-setup)"

type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	HTMLURL    string        `json:"html_url"`
	Prerelease bool          `json:"prerelease"`
	Draft      bool          `json:"draft"`
	Body       string        `json:"body"`
	Assets     []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type Result struct {
	Pin            manifest.Pin
	VendorFallback bool
	Reason         string
}

type Client struct {
	HTTP    *http.Client
	BaseAPI string
	Token   string
	Now     func() time.Time
}

func NewClient(token string) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 45 * time.Second},
		BaseAPI: "https://api.github.com",
		Token:   token,
		Now:     time.Now,
	}
}

func (c *Client) Latest(owner, repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", strings.TrimRight(c.BaseAPI, "/"), owner, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github latest %s/%s: HTTP %d: %s", owner, repo, res.StatusCode, truncate(string(body), 200))
	}
	var rel GitHubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if rel.Draft || rel.Prerelease {
		return nil, fmt.Errorf("github latest %s/%s is draft/prerelease; refusing to invent a stable version", owner, repo)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("github latest %s/%s has no tag", owner, repo)
	}
	return &rel, nil
}

func MatchAsset(rel *GitHubRelease, glob string) (GitHubAsset, error) {
	var hits []GitHubAsset
	for _, a := range rel.Assets {
		ok, err := path.Match(glob, a.Name)
		if err != nil {
			return GitHubAsset{}, err
		}
		if ok {
			hits = append(hits, a)
		}
	}
	if len(hits) == 0 {
		return GitHubAsset{}, fmt.Errorf("no asset matching %q on %s", glob, rel.TagName)
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	filtered := hits[:0]
	for _, a := range hits {
		n := strings.ToLower(a.Name)
		if strings.Contains(n, "portable") || strings.Contains(n, "mingit") || strings.Contains(n, "busybox") {
			continue
		}
		filtered = append(filtered, a)
	}
	if len(filtered) == 1 {
		return filtered[0], nil
	}
	return GitHubAsset{}, fmt.Errorf("asset glob %q matched %d files on %s; not guessing", glob, len(hits), rel.TagName)
}

func DigestSHA256(digest string) (string, bool) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return "", false
	}
	if strings.HasPrefix(digest, "sha256:") {
		hex := strings.TrimPrefix(digest, "sha256:")
		if looksSHA256(hex) {
			return strings.ToLower(hex), true
		}
	}
	return "", false
}

func PinFromGitHub(rel *GitHubRelease, glob string, now time.Time) (manifest.Pin, error) {
	asset, err := MatchAsset(rel, glob)
	if err != nil {
		return manifest.Pin{}, err
	}
	sha, ok := DigestSHA256(asset.Digest)
	if !ok {
		sha, ok = SHA256FromBody(rel.Body, asset.Name)
	}
	if !ok {
		return manifest.Pin{}, fmt.Errorf("no sha256 for %s on %s", asset.Name, rel.TagName)
	}
	return manifest.Pin{
		Version:    rel.TagName,
		URL:        asset.BrowserDownloadURL,
		SHA256:     sha,
		Size:       asset.Size,
		AssetName:  asset.Name,
		ReleaseURL: rel.HTMLURL,
		ProvenAt:   now.UTC().Format(time.RFC3339),
	}, nil
}

func SHA256FromBody(body, fileName string) (string, bool) {
	base := path.Base(fileName)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && looksSHA256(fields[0]) && strings.HasSuffix(fields[1], base) {
			return strings.ToLower(fields[0]), true
		}
		if len(fields) >= 2 && looksSHA256(fields[len(fields)-1]) && strings.Contains(fields[0], base) {
			return strings.ToLower(fields[len(fields)-1]), true
		}
		if strings.Contains(line, "|") {
			parts := strings.Split(line, "|")
			var cells []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cells = append(cells, p)
				}
			}
			if len(cells) >= 2 && strings.Contains(cells[0], base) && looksSHA256(cells[1]) {
				return strings.ToLower(cells[1]), true
			}
		}
	}
	return "", false
}

func looksSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !ok {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
