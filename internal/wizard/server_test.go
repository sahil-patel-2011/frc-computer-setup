package wizard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/engine"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func TestCatalogAndStatic(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{Win: true}
	r := &engine.Runner{Catalog: c, Host: h, Demo: true, Emit: func(engine.Event) {}}
	s := New(c, r, h, true)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), "Computer setup") {
		t.Fatalf("index %d %s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), "btn-go") {
		t.Fatal("missing Install button")
	}
	if !strings.Contains(string(body), "Nothing installs unless you check it") {
		t.Fatal("missing opt-in copy")
	}

	res, err = http.Get(ts.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var cat struct {
		Season int    `json:"season"`
		OS     string `json:"os"`
		Tools  []struct {
			ID        string `json:"id"`
			Selected  bool   `json:"selected"`
			Kind      string `json:"kind"`
			Available bool   `json:"available"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(res.Body).Decode(&cat); err != nil {
		t.Fatal(err)
	}
	if cat.Season != 2026 {
		t.Fatalf("season %d", cat.Season)
	}
	found := map[string]bool{}
	for _, tool := range cat.Tools {
		found[tool.ID] = true
		if tool.Selected {
			t.Fatalf("%s must start unchecked", tool.ID)
		}
		if tool.ID == "ni-game-tools" && (tool.Kind != "vendor_page" || !tool.Available) {
			t.Fatalf("NI on Windows %+v", tool)
		}
		if tool.ID == "vscode" && tool.Kind != "download" {
			t.Fatalf("vscode %+v", tool)
		}
	}
	for _, id := range []string{"git", "wpilib", "ni-game-tools", "pathplanner", "advantagescope", "choreo", "vscode", "limelight", "robot-code-6925"} {
		if !found[id] {
			t.Fatalf("missing %s", id)
		}
	}
}

func TestCatalogLinuxHidesDriverStation(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{OS: "linux", Arch: "amd64"}
	r := &engine.Runner{Catalog: c, Host: h, Demo: true, Emit: func(engine.Event) {}}
	s := New(c, r, h, true)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var cat struct {
		OS    string `json:"os"`
		Tools []struct {
			ID        string `json:"id"`
			Selected  bool   `json:"selected"`
			Kind      string `json:"kind"`
			Available bool   `json:"available"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(res.Body).Decode(&cat); err != nil {
		t.Fatal(err)
	}
	if cat.OS != "linux" {
		t.Fatalf("os %s", cat.OS)
	}
	for _, tool := range cat.Tools {
		if tool.Selected {
			t.Fatalf("%s must start unchecked", tool.ID)
		}
		switch tool.ID {
		case "ni-game-tools":
			if tool.Available || tool.Kind != "windows_only" {
				t.Fatalf("NI on linux %+v", tool)
			}
		case "wpilib":
			if !tool.Available || tool.Kind != "download" {
				t.Fatalf("wpilib on linux %+v", tool)
			}
		case "git":
			if tool.Kind != "vendor_page" {
				t.Fatalf("git on linux %+v", tool)
			}
		}
	}
}

func TestCatalogDarwinWPILibDownload(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{OS: "darwin", Arch: "arm64"}
	r := &engine.Runner{Catalog: c, Host: h, Demo: true, Emit: func(engine.Event) {}}
	s := New(c, r, h, true)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var cat struct {
		OS    string `json:"os"`
		Tools []struct {
			ID        string `json:"id"`
			Selected  bool   `json:"selected"`
			Kind      string `json:"kind"`
			Available bool   `json:"available"`
			Version   string `json:"version"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(res.Body).Decode(&cat); err != nil {
		t.Fatal(err)
	}
	if cat.OS != "darwin" {
		t.Fatalf("os %s", cat.OS)
	}
	for _, tool := range cat.Tools {
		if tool.Selected {
			t.Fatalf("%s must start unchecked", tool.ID)
		}
		switch tool.ID {
		case "ni-game-tools":
			if tool.Available || tool.Kind != "windows_only" {
				t.Fatalf("NI on darwin %+v", tool)
			}
		case "wpilib":
			if !tool.Available || tool.Kind != "download" || tool.Version != "v2026.2.1" {
				t.Fatalf("wpilib on darwin %+v", tool)
			}
		case "advantagescope", "choreo", "pathplanner":
			if !tool.Available || tool.Kind != "download" {
				t.Fatalf("%s on darwin %+v", tool.ID, tool)
			}
		case "git":
			if tool.Kind != "vendor_page" {
				t.Fatalf("git on darwin %+v", tool)
			}
		default:
		}
	}
}

func TestRunDemo(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{Win: true}
	r := &engine.Runner{Catalog: c, Host: h, Demo: true, Emit: func(engine.Event) {}}
	s := New(c, r, h, true)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/run", "application/json", strings.NewReader(`{"toolIds":["git"]}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("run %d", res.StatusCode)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.ElevateCalls >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if h.ElevateCalls != 1 {
		t.Fatalf("wizard run should elevate once, got %d", h.ElevateCalls)
	}
}

func TestRunOnlyPostedTools(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{Win: true}
	seen := map[string]bool{}
	r := &engine.Runner{Catalog: c, Host: h, Demo: true, Emit: func(e engine.Event) {
		if e.ToolID != "" {
			seen[e.ToolID] = true
		}
	}}
	s := New(c, r, h, true)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/run", "application/json", strings.NewReader(`{"toolIds":["git"]}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("run %d", res.StatusCode)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if seen["git"] && h.ElevateCalls >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !seen["git"] {
		t.Fatal("expected git to run")
	}
	for id := range seen {
		if id != "git" {
			t.Fatalf("unchecked tool ran: %s", id)
		}
	}
}
