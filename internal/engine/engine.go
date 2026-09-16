package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/download"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

type Phase string

const (
	PhaseCheck    Phase = "check"
	PhaseElevate  Phase = "elevate"
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

	gotElevation bool
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

func CanInstallSilently(tool manifest.Tool) bool {
	if tool.Kind == "git_clone" {
		return true
	}
	inst := tool.Install
	if inst == nil {
		return false
	}
	switch inst.Type {
	case "git_clone", "dmg", "zip", "tarball", "appimage":
		return true
	case "exe", "wpilib_iso", "wpilib_dmg", "wpilib_tarball":
		return host.HasSilentFlags(inst.Args)
	default:
		return false
	}
}

func (r *Runner) Run(ids []string) []ToolResult {
	out := make([]ToolResult, 0, len(ids))
	if r.Catalog == nil {
		for _, id := range ids {
			out = append(out, ToolResult{Tool: manifest.Tool{ID: id}, Status: StatusFailed, Message: "no catalog"})
		}
		return out
	}
	for _, id := range ids {
		tool, ok := r.Catalog.Tool(id)
		if !ok {
			r.event(Event{ToolID: id, Phase: PhaseError, Message: "Unknown tool", Err: "unknown"})
			out = append(out, ToolResult{Tool: manifest.Tool{ID: id}, Status: StatusFailed, Message: "unknown"})
			continue
		}
		out = append(out, r.RunTool(tool, nil))
	}
	return out
}

func (r *Runner) RunTool(tool manifest.Tool, _ <-chan string) ToolResult {
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
	det := DetectTool(r.Host, tool)
	if tool.Kind == "git_clone" {
		return r.runClone(tool, det)
	}
	if det.Installed && det.Current {
		msg := "Already current (" + det.Version + "). Next."
		if det.Version == "" {
			msg = "Already current. Next."
		}
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: msg, Already: true})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "already installed"}
	}
	if det.Installed && det.Version == "" {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Already on this computer. Could not prove a newer official build. Keeping it.", Already: true})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "already installed"}
	}
	if det.Installed && !det.Current {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseCheck, Message: "Found an older copy. Updating silently because you checked it."})
	}

	switch tool.Kind {
	case "vendor_page":
		return r.skipNeedsVendor(tool, tool.UnprovenReason)
	case "download":
		if !CanInstallSilently(tool) {
			return r.skipNeedsVendor(tool, "No silent installer flags. Needs the vendor page — not popping a wizard.")
		}
		return r.runDownload(tool)
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
		return r.skipNeedsVendor(tool, "No official silent installer for this computer.")
	case "git_clone":
		return r.runClone(tool, det)
	case "windows_only":
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseSkip, Message: "Windows only."})
		return ToolResult{Tool: tool, Status: StatusSkipped, Message: "windows-only"}
	default:
		msg := fmt.Sprintf("unknown kind %s", tool.Kind)
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: msg, Err: msg})
		return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
	}
}

func (r *Runner) skipNeedsVendor(tool manifest.Tool, why string) ToolResult {
	if why == "" {
		why = "No silent official installer. Needs the vendor page — not popping extra dialogs."
	}
	if tool.VendorURL == "" {
		tool.VendorURL = tool.DocsURL
	}
	msg := why
	if tool.VendorURL != "" && !strings.Contains(msg, tool.VendorURL) {
		msg = why + " " + tool.VendorURL
	}
	r.event(Event{
		ToolID:  tool.ID,
		Name:    tool.Name,
		Phase:   PhaseSkip,
		Message: "Needs vendor page: " + msg,
		URL:     tool.VendorURL,
	})
	return ToolResult{Tool: tool, Status: StatusSkipped, Message: "needs vendor page"}
}

func (r *Runner) runDownload(tool manifest.Tool) ToolResult {
	if tool.Pinned == nil {
		if tool.VendorURL == "" {
			tool.VendorURL = tool.DocsURL
		}
		return r.skipNeedsVendor(tool, "Pinned URL missing. Not inventing a version.")
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

	if err := r.ensureElevated(); err != nil {
		msg := err.Error()
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: "Admin was declined. Nothing more was installed. " + msg, Err: msg})
		return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
	}

	silentMsg := installMessage(tool.Install)
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseInstall, Message: silentMsg})
	if r.Demo {
		time.Sleep(200 * time.Millisecond)
	} else {
		if err := r.install(tool, dest); err != nil {
			if isNeedsVendor(err) {
				return r.skipNeedsVendor(tool, err.Error())
			}
			msg := err.Error()
			r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: "Installer did not finish: " + msg, Err: msg})
			return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
		}
	}

	return r.selfCheck(tool, dest)
}

func (r *Runner) selfCheck(tool manifest.Tool, dest string) ToolResult {
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseVerify, Message: "Checking that it landed on the laptop…"})
	if r.Demo {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Demo verify OK. Next."})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "demo"}
	}
	if tool.Verify == nil {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Installed. Next."})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "installed"}
	}
	if err := VerifyTool(r.Host, tool); err == nil {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Installed. Next."})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "installed"}
	}
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseInstall, Message: "Self-check missed it. Retrying once, still silent."})
	if err := r.install(tool, dest); err != nil {
		if isNeedsVendor(err) {
			return r.skipNeedsVendor(tool, err.Error())
		}
		msg := err.Error()
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: "Retry failed: " + msg, Err: msg})
		return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
	}
	if err := VerifyTool(r.Host, tool); err != nil {
		msg := "Self-check failed after a silent retry: " + err.Error()
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: msg, Err: msg})
		return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
	}
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Installed after one silent retry. Next."})
	return ToolResult{Tool: tool, Status: StatusOK, Message: "installed"}
}

func (r *Runner) runClone(tool manifest.Tool, det Detection) ToolResult {
	url := tool.CloneURL
	dest := tool.CloneDest
	if tool.Install != nil {
		if dest == "" {
			dest = tool.Install.Dest
		}
		if url == "" {
			url = tool.Install.URL
		}
	}
	if r.Host != nil && dest != "" {
		dest = r.Host.Expand(dest)
		if r.Host.Exists(filepath.Join(dest, ".git")) || r.Host.Exists(dest) {
			r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Robot code folder already exists. Next.", Already: true})
			return ToolResult{Tool: tool, Status: StatusOK, Message: "already installed"}
		}
	}
	if det.Installed {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Robot code folder already exists. Next.", Already: true})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "already installed"}
	}
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseInstall, Message: "Cloning the public 6925 robot code (no secrets)."})
	if r.Demo {
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Demo clone skipped. Next."})
		return ToolResult{Tool: tool, Status: StatusOK, Message: "demo"}
	}
	if err := r.Host.InstallKind("git_clone", dest, []string{url}); err != nil {
		msg := err.Error()
		r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseError, Message: "Clone failed: " + msg, Err: msg})
		return ToolResult{Tool: tool, Status: StatusFailed, Message: msg}
	}
	r.event(Event{ToolID: tool.ID, Name: tool.Name, Phase: PhaseDone, Message: "Cloned. Next."})
	return ToolResult{Tool: tool, Status: StatusOK, Message: "installed"}
}

func (r *Runner) ensureElevated() error {
	if r.Host != nil && !r.Host.IsWindows() {
		return nil
	}
	if r.gotElevation {
		return nil
	}
	r.gotElevation = true
	r.event(Event{Phase: PhaseElevate, Message: "Windows needs one admin yes. After that, this window does the rest."})
	if r.Host == nil {
		return nil
	}
	return r.Host.EnsureElevated()
}

func isNeedsVendor(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "needs vendor page")
}

func installMessage(inst *manifest.Install) string {
	if inst == nil {
		return "Installing silently. Stay on this step until it finishes."
	}
	switch inst.Type {
	case "exe":
		if host.HasSilentFlags(inst.Args) {
			return "Installing with the vendor’s silent flags. Stay on this step until it finishes."
		}
		return "Needs vendor page — this installer cannot run silently."
	case "dmg":
		return "Copying the official app into Applications (unattended). Stay on this step until it finishes."
	case "appimage":
		return "Installing the official AppImage into ~/.local/bin (unattended)."
	case "zip":
		return "Extracting the official zip (unattended)."
	case "tarball":
		return "Extracting the official archive (unattended)."
	case "git_clone":
		return "Cloning the public repository (no secrets)."
	case "wpilib_iso", "wpilib_dmg", "wpilib_tarball":
		if host.HasSilentFlags(inst.Args) {
			return "Installing WPILib silently. Stay on this step until it finishes."
		}
		return "Needs vendor page — WPILib has no silent installer on this computer."
	default:
		return "Installing silently. Stay on this step until it finishes."
	}
}

func (r *Runner) install(tool manifest.Tool, path string) error {
	kind := ""
	var args []string
	if tool.Install != nil {
		kind = tool.Install.Type
		args = tool.Install.Args
		if kind == "git_clone" {
			url := tool.Install.URL
			if url == "" {
				url = tool.CloneURL
			}
			dest := tool.Install.Dest
			if dest == "" {
				dest = tool.CloneDest
			}
			if r.Host != nil {
				dest = r.Host.Expand(dest)
			}
			return r.Host.InstallKind(kind, dest, []string{url})
		}
	}
	return r.Host.InstallKind(kind, path, args)
}

func DefaultSelected(_ *manifest.Catalog, _, _ string) []string {
	return nil
}
