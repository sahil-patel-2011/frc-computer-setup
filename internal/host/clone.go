package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func gitClone(url, dest string) error {
	if url == "" || dest == "" {
		return fmt.Errorf("git clone missing url or dest")
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		return nil
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is not on PATH — install Git first")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(git, "clone", "--depth", "1", url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
