package host

import (
	"archive/zip"
	"os"
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

func TestVerifyGlob(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "advantagescope-linux-x64-v26.0.2.AppImage")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Fake{Files: map[string]bool{}}
	pattern := filepath.Join(dir, "advantagescope-linux-*.AppImage")
	if err := Verify(h, []string{pattern}, nil); err != nil {
		t.Fatal(err)
	}
	if err := Verify(h, []string{filepath.Join(dir, "no-such-file")}, nil); err == nil {
		t.Fatal("literal missing path should not glob-match")
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

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app.zip")
	zf, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := extractZip(src, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.zip")
	zf, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("no"))
	_ = zw.Close()
	_ = zf.Close()
	if err := extractZip(src, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected traversal error")
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
