package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/download"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func sampleGit(url, sha string, size int64) manifest.Tool {
	return manifest.Tool{
		ID: "git", Name: "Git", Summary: "git", Why: "w", Group: "required", Kind: "download",
		Pinned:  &manifest.Pin{Version: "v1", URL: url, SHA256: sha, Size: size, AssetName: "Git-setup.exe"},
		Install: &manifest.Install{Type: "exe", Args: []string{"/VERYSILENT"}},
		Verify:  &manifest.Verify{Paths: []string{`C:\Program Files\Git\cmd\git.exe`}, Commands: []manifest.Command{{Name: "git", Args: []string{"--version"}}}},
	}
}

func TestSkipIfAlreadyInstalled(t *testing.T) {
	h := &host.Fake{Files: map[string]bool{`C:\Program Files\Git\cmd\git.exe`: true}, Win: true}
	var phases []Phase
	r := &Runner{Host: h, Emit: func(e Event) { phases = append(phases, e.Phase) }}
	res := r.RunTool(sampleGit("https://example.com/x", strings.Repeat("a", 64), 1), nil)
	if res.Status != StatusOK || res.Message != "already installed" {
		t.Fatalf("%+v", res)
	}
	if phases[len(phases)-1] != PhaseDone {
		t.Fatalf("%v", phases)
	}
}

func TestVendorPageDoesNotInventURL(t *testing.T) {
	h := &host.Fake{}
	tool := manifest.Tool{
		ID: "ni-game-tools", Name: "NI", Summary: "ds", Why: "w", Group: "required", Kind: "vendor_page",
		VendorURL:      "https://www.ni.com/en/support/downloads/drivers/download.frc-game-tools.html",
		UnprovenReason: "no public url",
	}
	ack := make(chan string, 1)
	ack <- "installed"
	r := &Runner{Host: h, Emit: func(Event) {}}
	res := r.RunTool(tool, ack)
	if res.Status != StatusVendor {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 1 || !strings.Contains(h.Opened[0], "ni.com") {
		t.Fatalf("opened %v", h.Opened)
	}
}

func TestDownloadInstallVerify(t *testing.T) {
	payload := []byte("installer-bytes")
	sum := sha256.Sum256(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	h := &host.Fake{Win: true, Files: map[string]bool{}}
	dl := download.New()
	dl.HTTP = srv.Client()
	var sawDownload bool
	r := &Runner{
		Host:  h,
		DL:    dl,
		Cache: t.TempDir(),
		Emit: func(e Event) {
			if e.Phase == PhaseDownload {
				sawDownload = true
			}
		},
	}
	tool := sampleGit(srv.URL+"/Git-setup.exe", hex.EncodeToString(sum[:]), int64(len(payload)))
	// intercept StartWait via Fake.Started; then mark file present before verify
	orig := r.Host
	fh := orig.(*host.Fake)
	wrapper := &mutateHost{Fake: fh, afterStart: func() {
		fh.Files[`C:\Program Files\Git\cmd\git.exe`] = true
		if fh.Bins == nil {
			fh.Bins = map[string]string{}
		}
		fh.Bins["git"] = `C:\Program Files\Git\cmd\git.exe`
	}}
	r.Host = wrapper
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK {
		t.Fatalf("%+v started=%v", res, fh.Started)
	}
	if !sawDownload {
		t.Fatal("expected download phase")
	}
	if len(fh.Started) != 1 {
		t.Fatalf("started %v", fh.Started)
	}
}

type mutateHost struct {
	*host.Fake
	afterStart func()
}

func (m *mutateHost) StartWait(path string, args []string) error {
	err := m.Fake.StartWait(path, args)
	if m.afterStart != nil {
		m.afterStart()
	}
	return err
}

func TestDemoDoesNotHitNetwork(t *testing.T) {
	h := &host.Fake{Win: true}
	r := &Runner{Host: h, Demo: true, Emit: func(Event) {}}
	tool := sampleGit("https://example.invalid/nope.exe", strings.Repeat("b", 64), 99)
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK {
		t.Fatalf("%+v", res)
	}
}

func TestSkipWindowsOnly(t *testing.T) {
	h := &host.Fake{OS: "linux"}
	tool := manifest.Tool{
		ID: "ni-game-tools", Name: "NI", Summary: "ds", Why: "w", Group: "required",
		Kind: "vendor_page", WindowsOnly: true, Platforms: []string{"windows"},
		VendorURL: "https://www.ni.com/en/support/downloads/drivers/download.frc-game-tools.html",
	}
	r := &Runner{Host: h, Emit: func(Event) {}}
	res := r.RunTool(tool, nil)
	if res.Status != StatusSkipped {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 0 {
		t.Fatalf("should not open NI page on linux: %v", h.Opened)
	}
}

func TestLinuxGitIsVendorNotInvented(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	git, ok := c.Tool("git")
	if !ok {
		t.Fatal("missing git")
	}
	h := &host.Fake{OS: "linux", Arch: "amd64"}
	ack := make(chan string, 1)
	ack <- "skip"
	r := &Runner{Host: h, Catalog: c, Emit: func(Event) {}}
	res := r.RunTool(git, ack)
	if res.Status != StatusSkipped {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 1 || !strings.Contains(h.Opened[0], "git-scm.com/download/linux") {
		t.Fatalf("opened %v", h.Opened)
	}
}

func TestDefaultSelectedLinuxOmitsDriverStation(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	ids := DefaultSelected(c, "linux", "amd64")
	for _, id := range ids {
		if id == "ni-game-tools" || id == "labview" || id == "radio-om5p" {
			t.Fatalf("windows-only %s selected on linux", id)
		}
	}
	joined := strings.Join(ids, ",")
	for _, need := range []string{"git", "wpilib", "pathplanner", "advantagescope", "choreo"} {
		if !strings.Contains(joined, need) {
			t.Fatalf("missing %s in %v", need, ids)
		}
	}
}

func TestInstallMessageSilent(t *testing.T) {
	msg := installMessage(&manifest.Install{Type: "exe", Args: []string{"/S"}})
	if !strings.Contains(msg, "silent") {
		t.Fatal(msg)
	}
	msg = installMessage(&manifest.Install{Type: "wpilib_iso"})
	if !strings.Contains(msg, "WPILib") {
		t.Fatal(msg)
	}
	msg = installMessage(&manifest.Install{Type: "appimage"})
	if !strings.Contains(msg, "unattended") {
		t.Fatal(msg)
	}
}

func TestWPILibISOInstallType(t *testing.T) {
	h := &host.Fake{Win: true}
	r := &Runner{Host: h, Demo: true, Emit: func(Event) {}}
	tool := manifest.Tool{
		ID: "wpilib", Name: "WPILib", Summary: "s", Why: "w", Group: "required", Kind: "download",
		Pinned:  &manifest.Pin{Version: "v2026.2.1", URL: "https://example.invalid/x.iso", SHA256: strings.Repeat("c", 64), AssetName: "x.iso"},
		Install: &manifest.Install{Type: "wpilib_iso"},
		Verify:  &manifest.Verify{Paths: []string{`C:\Users\Public\wpilib\2026`}},
	}
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK {
		t.Fatalf("%+v", res)
	}
}

func TestMissingPinBecomesVendor(t *testing.T) {
	h := &host.Fake{}
	ack := make(chan string, 1)
	ack <- "skip"
	r := &Runner{Host: h, Emit: func(Event) {}}
	tool := manifest.Tool{ID: "x", Name: "X", Summary: "s", Why: "w", Group: "required", Kind: "download", DocsURL: "https://docs.wpilib.org"}
	res := r.RunTool(tool, ack)
	if res.Status != StatusSkipped {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 1 {
		t.Fatalf("opened %v", h.Opened)
	}
}
