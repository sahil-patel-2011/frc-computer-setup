package host

import "os"

type Fake struct {
	Files        map[string]bool
	Bins         map[string]string
	Runs         []string
	Opened       []string
	Started      []string
	StartedArgs  [][]string
	ISO          []string
	ISOArgs      [][]string
	Win          bool
	OS           string
	Arch         string
	FailRun      bool
	FailStart    bool
	FailElevate  bool
	ElevateCalls int
	Elevated     bool
	RunOut       map[string]string
}

func (f *Fake) Expand(path string) string {
	return os.ExpandEnv(path)
}

func (f *Fake) Exists(path string) bool {
	if f.Files[path] || f.Files[f.Expand(path)] {
		return true
	}
	return false
}

func (f *Fake) LookPath(name string) (string, error) {
	if p, ok := f.Bins[name]; ok {
		return p, nil
	}
	return "", os.ErrNotExist
}

func (f *Fake) Run(name string, args ...string) (string, error) {
	f.Runs = append(f.Runs, name)
	if f.FailRun {
		return "", os.ErrNotExist
	}
	if f.RunOut != nil {
		if out, ok := f.RunOut[name]; ok {
			return out, nil
		}
	}
	return "ok", nil
}

func (f *Fake) StartWait(path string, args []string) error {
	f.Started = append(f.Started, path)
	cp := append([]string(nil), args...)
	f.StartedArgs = append(f.StartedArgs, cp)
	if f.FailStart {
		return os.ErrPermission
	}
	return nil
}

func (f *Fake) OpenURL(url string) error {
	f.Opened = append(f.Opened, url)
	return nil
}

func (f *Fake) InstallWPILibISO(isoPath string, args []string) error {
	f.ISO = append(f.ISO, isoPath)
	f.ISOArgs = append(f.ISOArgs, append([]string(nil), args...))
	if f.FailStart {
		return os.ErrPermission
	}
	return nil
}

func (f *Fake) InstallKind(kind, path string, args []string) error {
	switch kind {
	case "wpilib_iso", "wpilib_dmg", "wpilib_tarball":
		return f.InstallWPILibISO(path, args)
	case "git_clone":
		f.Started = append(f.Started, "git-clone:"+path)
		f.StartedArgs = append(f.StartedArgs, append([]string(nil), args...))
		if f.FailStart {
			return os.ErrPermission
		}
		return nil
	default:
		return f.StartWait(path, args)
	}
}

func (f *Fake) EnsureElevated() error {
	f.ElevateCalls++
	if f.FailElevate {
		return os.ErrPermission
	}
	f.Elevated = true
	return nil
}

func (f *Fake) IsWindows() bool { return f.Win }

func (f *Fake) GOOS() string {
	if f.Win {
		return "windows"
	}
	if f.OS != "" {
		return f.OS
	}
	return "linux"
}

func (f *Fake) GOARCH() string {
	if f.Arch != "" {
		return f.Arch
	}
	return "amd64"
}
