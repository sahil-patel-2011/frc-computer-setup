package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type InstallerResult struct {
	Skipped bool
	Message string
}

type Host interface {
	Expand(path string) string
	Exists(path string) bool
	LookPath(name string) (string, error)
	Run(name string, args ...string) (string, error)
	StartWait(path string, args []string) error
	OpenURL(url string) error
	InstallWPILibISO(isoPath string) error
	IsWindows() bool
}

type Real struct {
	Look    func(string) (string, error)
	RunCmd  func(name string, args ...string) (string, error)
	Start   func(path string, args []string) error
	Open    func(url string) error
	ISO     func(isoPath string) error
	Windows bool
	Now     func() time.Time
}

func NewReal() *Real {
	return &Real{
		Look:    exec.LookPath,
		Windows: isWindows(),
		Now:     time.Now,
	}
}

func (h *Real) Expand(path string) string {
	return os.ExpandEnv(path)
}

func (h *Real) Exists(path string) bool {
	p := h.Expand(path)
	st, err := os.Stat(p)
	return err == nil && st != nil
}

func (h *Real) LookPath(name string) (string, error) {
	if h.Look != nil {
		return h.Look(name)
	}
	return exec.LookPath(name)
}

func (h *Real) Run(name string, args ...string) (string, error) {
	if h.RunCmd != nil {
		return h.RunCmd(name, args...)
	}
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func (h *Real) StartWait(path string, args []string) error {
	if h.Start != nil {
		return h.Start(path, args)
	}
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (h *Real) OpenURL(url string) error {
	if h.Open != nil {
		return h.Open(url)
	}
	return openURL(url)
}

func (h *Real) InstallWPILibISO(isoPath string) error {
	if h.ISO != nil {
		return h.ISO(isoPath)
	}
	return installWPILibISO(isoPath)
}

func (h *Real) IsWindows() bool {
	return h.Windows
}

func Verify(h Host, paths []string, commands []struct {
	Name string
	Args []string
}) error {
	for _, p := range paths {
		if h.Exists(p) {
			return nil
		}
		// directory or glob-ish year folder: treat as success if any child exists
		exp := h.Expand(p)
		if matches, _ := filepath.Glob(exp); len(matches) > 0 {
			return nil
		}
	}
	for _, c := range commands {
		name, err := h.LookPath(c.Name)
		if err != nil {
			continue
		}
		if _, err := h.Run(name, c.Args...); err == nil {
			return nil
		}
	}
	if len(paths) == 0 && len(commands) == 0 {
		return fmt.Errorf("nothing to verify")
	}
	return fmt.Errorf("not found yet")
}

func WhichGit(h Host) string {
	if p, err := h.LookPath("git"); err == nil {
		return p
	}
	return ""
}

func Quote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
