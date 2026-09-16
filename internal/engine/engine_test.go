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
		Install: &manifest.Install{Type: "exe", Args: []string{"/VERYSILENT", "/NORESTART", "/ALLUSERS"}},
		Verify:  &manifest.Verify{Paths: []string{`C:\Program Files\Git\cmd\git.exe`}, Commands: []manifest.Command{{Name: "git", Args: []string{"--version"}}}},
	}
}

func sampleOther(id, url, sha string, size int64) manifest.Tool {
	t := sampleGit(url, sha, size)
	t.ID = id
	t.Name = id
	t.Verify = &manifest.Verify{Paths: []string{`C:\Program Files\` + id + `\app.exe`}}
	return t
}

func TestSkipIfAlreadyInstalled(t *testing.T) {
	h := &host.Fake{Files: map[string]bool{`C:\Program Files\Git\cmd\git.exe`: true}, Win: true}
	var phases []Phase
	r := &Runner{Host: h, Emit: func(e Event) { phases = append(phases, e.Phase) }}
	res := r.RunTool(sampleGit("https://example.com/x", strings.Repeat("a", 64), 1), nil)
	if res.Status != StatusOK || res.Message != "already installed" {
		t.Fatalf("%+v", res)
	}
	last := phases[len(phases)-1]
	if last != PhaseDone && last != PhaseSkip {
		t.Fatalf("%v", phases)
	}
	if h.ElevateCalls != 0 {
		t.Fatalf("already-current must not elevate: %d", h.ElevateCalls)
	}
}

func TestVendorPageDoesNotInventURL(t *testing.T) {
	h := &host.Fake{Win: true}
	tool := manifest.Tool{
		ID: "ni-game-tools", Name: "NI", Summary: "ds", Why: "w", Group: "required", Kind: "vendor_page",
		VendorURL:      "https://www.ni.com/en/support/downloads/drivers/download.frc-game-tools.html",
		UnprovenReason: "no public url",
	}
	r := &Runner{Host: h, Emit: func(Event) {}}
	res := r.RunTool(tool, nil)
	if res.Status != StatusSkipped || res.Message != "needs vendor page" {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 0 {
		t.Fatalf("must not pop vendor page: %v", h.Opened)
	}
	if h.ElevateCalls != 0 {
		t.Fatalf("vendor skip must not elevate: %d", h.ElevateCalls)
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
	fh := h
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
	if len(fh.StartedArgs) != 1 || !containsArg(fh.StartedArgs[0], "/VERYSILENT") {
		t.Fatalf("silent args %v", fh.StartedArgs)
	}
	if fh.ElevateCalls != 1 {
		t.Fatalf("elevate once, got %d", fh.ElevateCalls)
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

func (m *mutateHost) InstallKind(kind, path string, args []string) error {
	err := m.Fake.InstallKind(kind, path, args)
	if m.afterStart != nil {
		m.afterStart()
	}
	return err
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestDemoDoesNotHitNetwork(t *testing.T) {
	h := &host.Fake{Win: true}
	r := &Runner{Host: h, Demo: true, Emit: func(Event) {}}
	tool := sampleGit("https://example.invalid/nope.exe", strings.Repeat("b", 64), 99)
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK {
		t.Fatalf("%+v", res)
	}
	if h.ElevateCalls != 1 {
		t.Fatalf("demo still requests elevation once, got %d", h.ElevateCalls)
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
	r := &Runner{Host: h, Catalog: c, Emit: func(Event) {}}
	res := r.RunTool(git, nil)
	if res.Status != StatusSkipped || res.Message != "needs vendor page" {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 0 {
		t.Fatalf("must not pop git vendor page: %v", h.Opened)
	}
}

func TestDefaultSelectedIsEmpty(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	ids := DefaultSelected(c, "linux", "amd64")
	if len(ids) != 0 {
		t.Fatalf("checklist starts empty, got %v", ids)
	}
	ids = DefaultSelected(c, "windows", "amd64")
	if len(ids) != 0 {
		t.Fatalf("checklist starts empty, got %v", ids)
	}
}

func TestInstallMessageSilent(t *testing.T) {
	msg := installMessage(&manifest.Install{Type: "exe", Args: []string{"/S"}})
	if !strings.Contains(msg, "silent") {
		t.Fatal(msg)
	}
	msg = installMessage(&manifest.Install{Type: "wpilib_iso", Args: []string{"--force", "-y"}})
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
		Install: &manifest.Install{Type: "wpilib_iso", Args: []string{"--force", "-y", "--install-mode", "all"}},
		Verify:  &manifest.Verify{Paths: []string{`C:\Users\Public\wpilib\2026`}},
	}
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK {
		t.Fatalf("%+v", res)
	}
	if !CanInstallSilently(tool) {
		t.Fatal("wpilib with --force must count as silent-capable")
	}
}

func TestMissingPinBecomesVendor(t *testing.T) {
	h := &host.Fake{}
	r := &Runner{Host: h, Emit: func(Event) {}}
	tool := manifest.Tool{ID: "x", Name: "X", Summary: "s", Why: "w", Group: "required", Kind: "download", DocsURL: "https://docs.wpilib.org"}
	res := r.RunTool(tool, nil)
	if res.Status != StatusSkipped || res.Message != "needs vendor page" {
		t.Fatalf("%+v", res)
	}
	if len(h.Opened) != 0 {
		t.Fatalf("opened %v", h.Opened)
	}
}

func TestUncheckedToolsNotInstalled(t *testing.T) {
	payload := []byte("installer-bytes")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	h := &host.Fake{Win: true, Files: map[string]bool{}}
	dl := download.New()
	dl.HTTP = srv.Client()
	git := sampleGit(srv.URL+"/Git-setup.exe", sha, int64(len(payload)))
	other := sampleOther("pathplanner", srv.URL+"/pp.exe", sha, int64(len(payload)))
	catalog := &manifest.Catalog{SchemaVersion: 1, Season: 2026, Tools: []manifest.Tool{git, other}}
	wrapper := &mutateHost{Fake: h, afterStart: func() {
		h.Files[`C:\Program Files\Git\cmd\git.exe`] = true
	}}
	r := &Runner{Catalog: catalog, Host: wrapper, DL: dl, Cache: t.TempDir(), Emit: func(Event) {}}
	results := r.Run([]string{"git"})
	if len(results) != 1 || results[0].Status != StatusOK {
		t.Fatalf("%+v started=%v", results, h.Started)
	}
	if len(h.Started) != 1 {
		t.Fatalf("unchecked pathplanner must not install: %v", h.Started)
	}
	if h.ElevateCalls != 1 {
		t.Fatalf("elevate once, got %d", h.ElevateCalls)
	}
}

func TestElevationRequestedOnce(t *testing.T) {
	payload := []byte("installer-bytes")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	h := &host.Fake{Win: true, Files: map[string]bool{}}
	dl := download.New()
	dl.HTTP = srv.Client()
	git := sampleGit(srv.URL+"/Git-setup.exe", sha, int64(len(payload)))
	other := sampleOther("pathplanner", srv.URL+"/pp.exe", sha, int64(len(payload)))
	n := 0
	wrapper := &mutateHost{Fake: h, afterStart: func() {
		n++
		if n == 1 {
			h.Files[`C:\Program Files\Git\cmd\git.exe`] = true
		}
		if n >= 2 {
			h.Files[`C:\Program Files\pathplanner\app.exe`] = true
		}
	}}
	r := &Runner{
		Catalog: &manifest.Catalog{SchemaVersion: 1, Season: 2026, Tools: []manifest.Tool{git, other}},
		Host:    wrapper, DL: dl, Cache: t.TempDir(), Emit: func(Event) {},
	}
	results := r.Run([]string{"git", "pathplanner"})
	if len(results) != 2 {
		t.Fatalf("%+v", results)
	}
	for _, res := range results {
		if res.Status != StatusOK {
			t.Fatalf("%+v started=%v", results, h.Started)
		}
	}
	if h.ElevateCalls != 1 {
		t.Fatalf("want one UAC for the whole run, got %d", h.ElevateCalls)
	}
	if len(h.Started) != 2 {
		t.Fatalf("started %v", h.Started)
	}
	for i, args := range h.StartedArgs {
		if !containsArg(args, "/VERYSILENT") {
			t.Fatalf("tool %d missing silent args: %v", i, args)
		}
	}
}

func TestSilentInstallArgsUsed(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	git, _ := c.Tool("git")
	inst := git.InstallFor("windows", "amd64")
	if inst == nil || !host.HasSilentFlags(inst.Args) || !containsArg(inst.Args, "/VERYSILENT") {
		t.Fatalf("git silent args %+v", inst)
	}
	if !containsArg(inst.Args, "/ALLUSERS") {
		t.Fatalf("elevated git should be ALLUSERS: %+v", inst)
	}
	vs, _ := c.Tool("vscode")
	vinst := vs.InstallFor("windows", "amd64")
	if vinst == nil || !host.HasSilentFlags(vinst.Args) {
		t.Fatalf("vscode silent args %+v", vinst)
	}
	if !CanInstallSilently(git.Resolve("windows", "amd64")) {
		t.Fatal("windows git must install silently")
	}
	ll, _ := c.Tool("limelight")
	resolved := ll.Resolve("windows", "amd64")
	if CanInstallSilently(resolved) {
		t.Fatal("limelight windows has no proven silent flags — skip, do not pop a wizard")
	}
}

func TestNoSilentExeSkipsVendorPage(t *testing.T) {
	h := &host.Fake{Win: true}
	r := &Runner{Host: h, Demo: true, Emit: func(Event) {}}
	tool := manifest.Tool{
		ID: "limelight", Name: "Limelight", Summary: "s", Why: "w", Group: "recommended", Kind: "download",
		VendorURL: "https://docs.limelightvision.io/docs/resources/downloads",
		Pinned:    &manifest.Pin{Version: "2.0.10", URL: "https://example.invalid/ll.exe", SHA256: strings.Repeat("d", 64), Size: 9},
		Install:   &manifest.Install{Type: "exe"},
	}
	res := r.RunTool(tool, nil)
	if res.Status != StatusSkipped || res.Message != "needs vendor page" {
		t.Fatalf("%+v", res)
	}
	if len(h.Started) != 0 {
		t.Fatalf("must not launch interactive installer: %v", h.Started)
	}
}

func TestSelfCheckRetriesOnce(t *testing.T) {
	payload := []byte("installer-bytes")
	sum := sha256.Sum256(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	h := &host.Fake{Win: true, Files: map[string]bool{}}
	dl := download.New()
	dl.HTTP = srv.Client()
	r := &Runner{Host: h, DL: dl, Cache: t.TempDir(), Emit: func(Event) {}}
	tool := sampleGit(srv.URL+"/Git-setup.exe", hex.EncodeToString(sum[:]), int64(len(payload)))
	res := r.RunTool(tool, nil)
	if res.Status != StatusFailed {
		t.Fatalf("want failed after retry, got %+v", res)
	}
	if len(h.Started) != 2 {
		t.Fatalf("want install + one silent retry, got %v", h.Started)
	}
	if h.ElevateCalls != 1 {
		t.Fatalf("retry must reuse the same elevation, got %d", h.ElevateCalls)
	}
}

func TestCanInstallSilently(t *testing.T) {
	if !CanInstallSilently(manifest.Tool{Kind: "git_clone"}) {
		t.Fatal("clone")
	}
	if CanInstallSilently(manifest.Tool{Kind: "download", Install: &manifest.Install{Type: "exe"}}) {
		t.Fatal("exe without flags")
	}
	if !CanInstallSilently(manifest.Tool{Kind: "download", Install: &manifest.Install{Type: "exe", Args: []string{"/S"}}}) {
		t.Fatal("/S")
	}
}
