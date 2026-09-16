package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/download"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

type Phase string

const (
	PhaseCheck    Phase = "check"
	PhaseDownload Phase = "download"
	PhaseInstall  Phase = "install"
	PhaseVerify   Phase = "verify"
	PhaseVendor   Phase = "vendor"
	PhaseDone     Phase = "done"
	PhaseSkip     Phase = "skip"
	PhaseError    Phase = "error"
)

type Event struct {
	ToolID  string `json:"toolId"`
	Name    string `json:"name"`
	Phase   Phase  `json:"phase"`
	Message string `json:"message"`
	Got     int64  `json:"got,omitempty"`
	Total   int64  `json:"total,omitempty"`
	URL     string `json:"url,omitempty"`
	NeedAck bool   `json:"needAck,omitempty"`
	AckKind string `json:"ackKind,omitempty"`
	Err     string `json:"error,omitempty"`
	Already bool   `json:"already,omitempty"`
}

type Status string

const (
	StatusOK      Status = "ok"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
	StatusVendor  Status = "vendor"
)

type ToolResult struct {
	Tool    manifest.Tool
	Status  Status
	Message string
}

type Runner struct {
	Catalog *manifest.Catalog
	Host    host.Host
	DL      *download.Client
	Cache   string
	Demo    bool
	Emit    func(Event)
}

func (r *Runner) event(e Event) {
	if r.Emit != nil {
		r.Emit(e)
	}
}

func (r *Runner) osArch() (string, string) {
	if r.Host == nil {
		return "linux", "amd64"
	}
	return r.Host.GOOS(), r.Host.GOARCH()
}

func (r *Runner) AlreadyInstalled(tool manifest.Tool) bool {
	return VerifyTool(r.Host, tool) == nil
}

func VerifyTool(h host.Host, tool manifest.Tool) error {
	if tool.Verify == nil {
		return fmt.Errorf("no verify checks")
	}
	cmds := make([]struct {
		Name string
		Args []string
	}, 0, len(tool.Verify.Commands))
	for _, c := range tool.Verify.Commands {
		cmds = append(cmds, struct {
			Name string
			Args []string
		}{Name: c.Name, Args: c.Args})
	}
	return host.Verify(h, tool.Verify.Paths, cmds)
}

func (r *Runner) RunTool(tool manifest.Tool, ack <-chan string) ToolResult {
	goos, goarch := r.osArch()
	if !tool.AvailableOn(goos) {
		msg := "Windows only — FRC Driver Station does not run on this computer."
		if tool.ID != "ni-game-tools" {
			msg = "This tool is Windows-only. Skipping on " + goos + "."
		}
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseSkip, Message: msg})
		return ToolResult{Tool: tool, Status: StatusSkipped, Message: "windows-only"}
	}
	tool = tool.Resolve(goos, goarch)
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseCheck, Message: "Checking whether this is already on the computer…"})
	if r.AlreadyInstalled(tool) {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Already installed. Next.", Already: true})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "already installed"}
	}

	switch tool.Kind {
	case "vendor_page":
		return r.runVendor(tool, ack)
	case "download":
		return r.runDownload(tool, ack)
	case "unavailable":
		if tool.VendorURL == "" {
			tool.VendorURL = tool.DocsURL
		}
		if tool.VendorURL == "" {
			msg := "No official installer for this computer. Not inventing a URL."
			r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseSkip, Message: msg})
			return ToolResult{Tool: tool, Status: StatusSkipped, Message: "unavailable"}
		}
		tool.Kind = "vendor_page"
		return r.runVendor(tool, ack)
	case "windows_only":
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseSkip, Message: "Windows only."})
		return ToolResult{Tool: tool, Status: StatusSkipped, Message: "windows-only"}
	default:
		msg := fmt.Sprintf("unknown kind %s", tool.Kind)
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: msg, Err: msg})
		return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
	}
}

func (r *Runner) runVendor(tool manifest.Tool, ack <-chan string) ToolResult {
	why := tool.UnprovenReason
	if why == "" {
		why = "No proven latest download URL. Opening the official vendor page instead of inventing a version."
	}
	r.event(Event{
		ToolID:  tool.ID,
		Name:    tool.Name,
		Phase:   PhaseVendor,
		Message: why,
		URL:     tool.VendorURL,
		NeedAck: true,
		AckKind: "vendor",
	})
	if !r.Demo {
		_ = r.Host.OpenURL(tool.VendorURL)
	}
	action := waitAck(ack, "installed")
	if action == "skip" {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseSkip, Message: "Skipped. You can do this later from the vendor page."})
		return ToolResult{Tool: tool, Status: StatusSkipped, Message: "skipped"}
	}
	if tool.Verify != nil && VerifyTool(r.Host, tool) != nil && !r.Demo {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseVerify, Message: "Could not see it on disk yet. If the vendor installer finished, continue anyway — some tools install to unusual folders."})
	}
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Vendor step recorded. Next."})
	return ToolResult{Tool: tool, Status: StatusVendor, Message: "vendor step"}
}

func (r *Runner) runDownload(tool manifest.Tool, ack <-chan string) ToolResult {
	if tool.Pinned == nil {
		tool.Kind = "vendor_page"
		if tool.VendorURL == "" {
			tool.VendorURL = tool.DocsURL
		}
		tool.UnprovenReason = "Pinned URL missing. Not inventing a version."
		return r.runVendor(tool, ack)
	}

	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDownload, Message: "Downloading the official installer…", Total: tool.Pinned.Size})
	var dest string
	if r.Demo {
		time.Sleep(200 * time.Millisecond)
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDownload, Message: "Demo mode — not downloading vendor binaries.", Got: tool.Pinned.Size, Total: tool.Pinned.Size})
		dest = filepath.Join(os.TempDir(), "frc-setup-demo-"+tool.ID)
	} else {
		cache := r.Cache
		if cache == "" {
			var err error
			cache, err = download.CacheDir()
			if err != nil {
				msg := err.Error()
				r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: msg, Err: msg})
				return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
			}
		}
		toolDir := filepath.Join(cache, tool.ID)
		got, err := r.DL.Fetch(*tool.Pinned, toolDir, func(got, total int64) {
			r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDownload, Message: "Downloading the official installer…", Got: got, Total: total})
		})
		if err != nil {
			msg := err.Error()
			r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: "Download failed: " + msg, Err: msg})
			return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
		}
		dest = got.Path
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDownload, Message: "Checksum matches the official release. Installing next.", Got: got.Size, Total: got.Size})
	}

	silentMsg := installMessage(tool.Install)
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseInstall, Message: silentMsg})
	if r.Demo {
		time.Sleep(200 * time.Millisecond)
	} else {
		if err := r.install(tool, dest); err != nil {
			msg := err.Error()
			r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: "Installer did not finish: " + msg, Err: msg})
			return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
		}
	}

	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseVerify, Message: "Checking that it landed on the laptop…"})
	if r.Demo {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Demo verify OK. Next."})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "demo"}
	}
	if tool.Verify != nil {
		if err := VerifyTool(r.Host, tool); err != nil {
			r.event(Event{
				ToolID:  tool.ID,
				Name:    tool.Name,
				Phase:   PhaseVerify,
				Message: "Could not auto-detect it yet. If the official installer finished, click Continue.",
				NeedAck: true,
				AckKind: "verify",
			})
			action := waitAck(ack, "continue")
			if action == "skip" {
				r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseSkip, Message: "Skipped verify."})
				return ToolResult{Tool: tool, Status: StatusSkipped, Message: "verify skipped"}
			}
		}
	}
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Installed. Next."})
	return ToolResult{Tool: tool, Status: StatusOK, Message: "installed"}
}

func installMessage(inst *manifest.Install) string {
	if inst == nil {
		return "Launching the official installer. Finish its screens, then this wizard continues."
	}
	switch inst.Type {
	case "exe":
		if len(inst.Args) > 0 {
			return "Installing with the vendor’s silent flags. Stay on this step until it finishes."
		}
		return "Launching the official installer. Finish its screens, then this wizard continues."
	case "dmg":
		return "Copying the official app into Applications (unattended). Stay on this step until it finishes."
	case "appimage":
		return "Installing the official AppImage into ~/.local/bin (unattended)."
	case "zip":
		return "Extracting the official zip (unattended)."
	case "tarball":
		return "Extracting the official archive (unattended)."
	case "wpilib_iso", "wpilib_dmg", "wpilib_tarball":
		return "Launching the official WPILib installer. Finish its screens, then this wizard continues."
	default:
		return "Launching the official installer. Finish its screens, then this wizard continues."
	}
}

func (r *Runner) install(tool manifest.Tool, path string) error {
	kind := ""
	var args []string
	if tool.Install != nil {
		kind = tool.Install.Type
		args = tool.Install.Args
	}
	return r.Host.InstallKind(kind, path, args)
}

func waitAck(ack <-chan string, defaultAction string) string {
	if ack == nil {
		return defaultAction
	}
	select {
	case v := <-ack:
		if v == "" {
			return defaultAction
		}
		return v
	case <-time.After(4 * time.Hour):
		return "skip"
	}
}

func DefaultSelected(c *manifest.Catalog, goos, goarch string) []string {
	var ids []string
	for _, t := range c.Tools {
		kind := t.EffectiveKind(goos, goarch)
		if kind == "windows_only" || kind == "unavailable" {
			continue
		}
		if t.SelectedByDefault() {
			ids = append(ids, t.ID)
		}
	}
	return ids
}
