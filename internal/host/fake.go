package host

import "os"

type Fake struct {
	Files     map[string]bool
	Bins      map[string]string
	Runs      []string
	Opened    []string
	Started   []string
	ISO       []string
	Win       bool
	FailRun   bool
	FailStart bool
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
	return "ok", nil
}

func (f *Fake) StartWait(path string, args []string) error {
	f.Started = append(f.Started, path)
	if f.FailStart {
		return os.ErrPermission
	}
	return nil
}

func (f *Fake) OpenURL(url string) error {
	f.Opened = append(f.Opened, url)
	return nil
}

func (f *Fake) InstallWPILibISO(isoPath string) error {
	f.ISO = append(f.ISO, isoPath)
	return nil
}

func (f *Fake) IsWindows() bool { return f.Win }
