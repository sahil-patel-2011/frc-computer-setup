package manifest

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed tools.json
var defaultFS embed.FS

func Default() (*Catalog, error) {
	data, err := defaultFS.ReadFile("tools.json")
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

type Catalog struct {
	SchemaVersion int    `json:"schemaVersion"`
	Season        int    `json:"season"`
	UpdatedAt     string `json:"updatedAt"`
	Notes         string `json:"notes,omitempty"`
	Tools         []Tool `json:"tools"`
}

type Tool struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Summary        string             `json:"summary"`
	Why            string             `json:"why"`
	Group          string             `json:"group"`
	Kind           string             `json:"kind"`
	Platforms      []string           `json:"platforms,omitempty"`
	WindowsOnly    bool               `json:"windowsOnly,omitempty"`
	DocsURL        string             `json:"docsUrl,omitempty"`
	VendorURL      string             `json:"vendorUrl,omitempty"`
	VendorByOS     map[string]string  `json:"vendorByOS,omitempty"`
	UnprovenReason string             `json:"unprovenReason,omitempty"`
	Source         *Source            `json:"source,omitempty"`
	Pinned         *Pin               `json:"pinned,omitempty"`
	Pins           map[string]Pin     `json:"pins,omitempty"`
	Install        *Install           `json:"install,omitempty"`
	Installs       map[string]Install `json:"installs,omitempty"`
	Verify         *Verify            `json:"verify,omitempty"`
	CloneURL       string             `json:"cloneUrl,omitempty"`
	CloneDest      string             `json:"cloneDest,omitempty"`
}

type Source struct {
	Type   string            `json:"type"`
	Owner  string            `json:"owner,omitempty"`
	Repo   string            `json:"repo,omitempty"`
	Asset  string            `json:"asset,omitempty"`
	Assets map[string]string `json:"assets,omitempty"`
}

type Pin struct {
	Version    string `json:"version"`
	URL        string `json:"url"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size,omitempty"`
	ETag       string `json:"etag,omitempty"`
	AssetName  string `json:"assetName,omitempty"`
	ReleaseURL string `json:"releaseUrl,omitempty"`
	ProvenAt   string `json:"provenAt,omitempty"`
}

type Install struct {
	Type string   `json:"type"`
	Args []string `json:"args,omitempty"`
	URL  string   `json:"url,omitempty"`
	Dest string   `json:"dest,omitempty"`
}

type Verify struct {
	Commands []Command `json:"commands,omitempty"`
	Paths    []string  `json:"paths,omitempty"`
}

type Command struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("manifest json: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Catalog) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schemaVersion %d (want 1)", c.SchemaVersion)
	}
	if len(c.Tools) == 0 {
		return fmt.Errorf("manifest has no tools")
	}
	seen := map[string]struct{}{}
	for i, t := range c.Tools {
		if t.ID == "" {
			return fmt.Errorf("tools[%d] missing id", i)
		}
		if _, ok := seen[t.ID]; ok {
			return fmt.Errorf("duplicate tool id %q", t.ID)
		}
		seen[t.ID] = struct{}{}
		if t.Name == "" || t.Summary == "" {
			return fmt.Errorf("tool %s needs name and summary", t.ID)
		}
		switch t.Kind {
		case "download":
			pins := t.AllPins()
			if len(pins) == 0 {
				return fmt.Errorf("tool %s is download but has no pinned url — use vendor_page instead of inventing one", t.ID)
			}
			for _, p := range pins {
				if err := validatePin(t.ID, p); err != nil {
					return err
				}
			}
		case "vendor_page":
			if t.VendorURL == "" && len(t.VendorByOS) == 0 {
				return fmt.Errorf("tool %s is vendor_page but missing vendorUrl", t.ID)
			}
		case "git_clone":
			if t.CloneURL == "" || !strings.HasPrefix(t.CloneURL, "https://") {
				return fmt.Errorf("tool %s git_clone needs an https cloneUrl — no secrets, public repo only", t.ID)
			}
			if t.CloneDest == "" && (t.Install == nil || t.Install.Dest == "") {
				return fmt.Errorf("tool %s git_clone needs cloneDest", t.ID)
			}
		default:
			return fmt.Errorf("tool %s has unknown kind %q", t.ID, t.Kind)
		}
		switch t.Group {
		case "required", "recommended", "optional":
		default:
			return fmt.Errorf("tool %s has unknown group %q", t.ID, t.Group)
		}
	}
	return nil
}

func (c *Catalog) Tool(id string) (Tool, bool) {
	for _, t := range c.Tools {
		if t.ID == id {
			return t, true
		}
	}
	return Tool{}, false
}

func (t Tool) IsDownload() bool {
	return t.Kind == "download"
}

func (t Tool) SelectedByDefault() bool {
	return t.Group == "required" || t.Group == "recommended"
}
