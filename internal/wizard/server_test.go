package wizard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/engine"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func TestCatalogAndStatic(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{}
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
	if !strings.Contains(string(body), "btn-begin") {
		t.Fatal("missing start button")
	}

	res, err = http.Get(ts.URL + "/api/catalog")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var cat struct {
		Season int `json:"season"`
		Tools  []struct {
			ID       string `json:"id"`
			Selected bool   `json:"selected"`
			Kind     string `json:"kind"`
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
		if tool.ID == "git" && !tool.Selected {
			t.Fatal("git should be selected by default")
		}
		if tool.ID == "ni-game-tools" && tool.Kind != "vendor_page" {
			t.Fatal("NI must stay a vendor page")
		}
	}
	for _, id := range []string{"git", "wpilib", "ni-game-tools", "pathplanner", "advantagescope", "choreo"} {
		if !found[id] {
			t.Fatalf("missing %s", id)
		}
	}
}

func TestRunDemo(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	h := &host.Fake{}
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
}
