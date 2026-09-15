package host

import (
	"path/filepath"
	"testing"
)

func TestVerifyPath(t *testing.T) {
	h := &Fake{Files: map[string]bool{"C:\\Users\\Public\\wpilib\\2026": true}}
	err := Verify(h, []string{"C:\\Users\\Public\\wpilib\\2026"}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCommand(t *testing.T) {
	h := &Fake{Bins: map[string]string{"git": "/usr/bin/git"}}
	err := Verify(h, nil, []struct {
		Name string
		Args []string
	}{{Name: "git", Args: []string{"--version"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyMissing(t *testing.T) {
	h := &Fake{Files: map[string]bool{}, Bins: map[string]string{}, FailRun: true}
	err := Verify(h, []string{filepath.Join("no", "such")}, []struct {
		Name string
		Args []string
	}{{Name: "git", Args: []string{"--version"}}})
	if err == nil {
		t.Fatal("expected missing")
	}
}
