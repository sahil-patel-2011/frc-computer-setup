package engine

import (
	"strings"
	"testing"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func TestNormalizeVersion(t *testing.T) {
	if NormalizeVersion("git version 2.55.0.windows.5") != "2.55.0" {
		t.Fatal(NormalizeVersion("git version 2.55.0.windows.5"))
	}
	if NormalizeVersion("v2026.2.1") != "2026.2.1" {
		t.Fatal(NormalizeVersion("v2026.2.1"))
	}
	if !VersionCurrent("1.137.0\n645f29cc", "1.137.0") {
		t.Fatal("vscode current")
	}
	if VersionCurrent("1.136.0", "1.137.0") {
		t.Fatal("old vscode should not look current")
	}
	if VersionCurrent("", "v1") {
		t.Fatal("unknown is not current")
	}
}

func TestDetectCurrentSkipsReinstall(t *testing.T) {
	h := &host.Fake{
		Win:   true,
		Files: map[string]bool{`C:\Program Files\Git\cmd\git.exe`: true},
		Bins:  map[string]string{"git": `C:\Program Files\Git\cmd\git.exe`},
		RunOut: map[string]string{
			`C:\Program Files\Git\cmd\git.exe`: "git version 2.55.0.windows.5",
		},
	}
	tool := sampleGit("https://example.invalid/Git.exe", strings.Repeat("a", 64), 1)
	tool.Pinned.Version = "v2.55.0.windows.5"
	r := &Runner{Host: h, Emit: func(Event) {}}
	res := r.RunTool(tool, nil)
	if res.Message != "already installed" {
		t.Fatalf("%+v", res)
	}
	if len(h.Started) != 0 {
		t.Fatalf("reinstalled: %v", h.Started)
	}
}

func TestDetectOffersUpdateSilently(t *testing.T) {
	h := &host.Fake{
		Win:   true,
		Files: map[string]bool{`C:\Program Files\Git\cmd\git.exe`: true},
		Bins:  map[string]string{"git": `C:\Program Files\Git\cmd\git.exe`},
		RunOut: map[string]string{
			`C:\Program Files\Git\cmd\git.exe`: "git version 2.40.0.windows.1",
		},
	}
	var kinds []string
	r := &Runner{Host: h, Demo: true, Emit: func(e Event) {
		if e.AckKind != "" {
			kinds = append(kinds, e.AckKind)
		}
		if e.NeedAck {
			t.Errorf("must not ask after checklist: %+v", e)
		}
	}}
	tool := sampleGit("https://example.invalid/Git.exe", strings.Repeat("a", 64), 1)
	tool.Pinned.Version = "v2.55.0.windows.5"
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK || res.Message != "demo" {
		t.Fatalf("%+v", res)
	}
	if len(kinds) != 0 {
		t.Fatalf("no extra yes/no, got ack kinds %v", kinds)
	}
}

func TestDetectUpdateProceedsInDemo(t *testing.T) {
	h := &host.Fake{
		Win:   true,
		Files: map[string]bool{`C:\Program Files\Git\cmd\git.exe`: true},
		Bins:  map[string]string{"git": `C:\Program Files\Git\cmd\git.exe`},
		RunOut: map[string]string{
			`C:\Program Files\Git\cmd\git.exe`: "git version 2.40.0.windows.1",
		},
	}
	r := &Runner{Host: h, Demo: true, Emit: func(Event) {}}
	tool := sampleGit("https://example.invalid/Git.exe", strings.Repeat("b", 64), 9)
	tool.Pinned.Version = "v2.55.0.windows.5"
	res := r.RunTool(tool, nil)
	if res.Status != StatusOK || res.Message != "demo" {
		t.Fatalf("%+v", res)
	}
}

func TestWPILibVSCodeDoesNotCountAsMicrosoft(t *testing.T) {
	h := &host.Fake{Win: true, Files: map[string]bool{
		`C:\Users\Public\wpilib\2026\vscode\Code.exe`: true,
	}}
	if !WPILibVSCodeInstalled(h, 2026) {
		t.Fatal("expected WPILib VS Code")
	}
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	vs, ok := c.Tool("vscode")
	if !ok {
		t.Fatal("missing vscode")
	}
	d := DetectTool(h, vs.Resolve("windows", "amd64"))
	if d.Installed {
		t.Fatal("WPILib VS Code must not count as standalone Microsoft VS Code")
	}
}

func TestCloneSkipsIfPresent(t *testing.T) {
	h := &host.Fake{Files: map[string]bool{
		"/tmp/6925/.git": true,
	}}
	tool := manifest.Tool{
		ID: "robot-code-6925", Name: "Clone", Summary: "s", Why: "w", Group: "optional",
		Kind: "git_clone", CloneURL: "https://github.com/sahil-patel-2011/6925-RobotCodeUpdated.git", CloneDest: "/tmp/6925",
		Install: &manifest.Install{Type: "git_clone", URL: "https://github.com/sahil-patel-2011/6925-RobotCodeUpdated.git", Dest: "/tmp/6925"},
		Verify:  &manifest.Verify{Paths: []string{"/tmp/6925/.git"}},
	}
	r := &Runner{Host: h, Emit: func(Event) {}}
	res := r.RunTool(tool, nil)
	if res.Message != "already installed" {
		t.Fatalf("%+v", res)
	}
	if len(h.Started) != 0 {
		t.Fatal("must not re-clone")
	}
}

func TestNoPhotonVisionInCatalog(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Tool("photonvision"); ok {
		t.Fatal("6925 uses Limelight, not PhotonVision — do not invent a PhotonVision install")
	}
	if _, ok := c.Tool("limelight"); !ok {
		t.Fatal("missing limelight")
	}
}

func TestVSCodeNotSelectedByDefault(t *testing.T) {
	c, err := manifest.Default()
	if err != nil {
		t.Fatal(err)
	}
	ids := DefaultSelected(c, "windows", "amd64")
	if len(ids) != 0 {
		t.Fatalf("nothing is checked unless the user checks it, got %v", ids)
	}
}
