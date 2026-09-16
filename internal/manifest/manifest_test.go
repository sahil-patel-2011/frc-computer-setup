package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePinnedCatalog(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if c.Season != 2026 {
		t.Fatalf("season %d", c.Season)
	}
	need := []string{"git", "wpilib", "ni-game-tools", "pathplanner", "advantagescope", "choreo"}
	for _, id := range need {
		tool, ok := c.Tool(id)
		if !ok {
			t.Fatalf("missing required catalog entry %s", id)
		}
		if id == "ni-game-tools" {
			if tool.Kind != "vendor_page" {
				t.Fatalf("ni-game-tools must be vendor_page, got %s", tool.Kind)
			}
			continue
		}
		if tool.Kind == "git_clone" {
			continue
		}
		if tool.Kind != "download" {
			t.Fatalf("%s want download, got %s", id, tool.Kind)
		}
		if tool.Pinned.SHA256 == "" {
			t.Fatalf("%s missing sha256", id)
		}
	}
}

func TestRejectInventedDownload(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 1,
		"season": 2026,
		"updatedAt": "2026-09-15T00:00:00Z",
		"tools": [{
			"id": "mystery",
			"name": "Mystery",
			"summary": "no",
			"why": "no",
			"group": "required",
			"kind": "download"
		}]
	}`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected error for download without pin")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error")
	}
}

func TestDuplicateID(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 1,
		"season": 2026,
		"updatedAt": "x",
		"tools": [
			{"id":"a","name":"A","summary":"s","why":"w","group":"required","kind":"vendor_page","vendorUrl":"https://example.com"},
			{"id":"a","name":"A2","summary":"s","why":"w","group":"required","kind":"vendor_page","vendorUrl":"https://example.com"}
		]
	}`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestHTTPURLRejected(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": 1,
		"season": 2026,
		"updatedAt": "x",
		"tools": [{
			"id":"bad",
			"name":"Bad",
			"summary":"s",
			"why":"w",
			"group":"required",
			"kind":"download",
			"pinned": {"version":"1","url":"http://example.com/x.exe","sha256":"aa"}
		}]
	}`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected https-only error")
	}
}

func TestWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := "tools.json"
	c, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "tools.json")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.Tools) != len(c.Tools) {
		t.Fatalf("tool count %d vs %d", len(c2.Tools), len(c.Tools))
	}
}
