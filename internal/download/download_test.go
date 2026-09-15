package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func TestFetchVerifiesSHA256(t *testing.T) {
	payload := []byte("wpilib-not-really")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	c := New()
	c.HTTP = srv.Client()
	dir := t.TempDir()
	res, err := c.Fetch(manifest.Pin{
		URL:       srv.URL + "/WPILib.iso",
		SHA256:    hexSum,
		Size:      int64(len(payload)),
		AssetName: "WPILib.iso",
		ETag:      `"v1"`,
	}, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA256 != hexSum {
		t.Fatalf("sum %s", res.SHA256)
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatal("payload mismatch")
	}

	res2, err := c.Fetch(manifest.Pin{
		URL:       srv.URL + "/WPILib.iso",
		SHA256:    hexSum,
		AssetName: "WPILib.iso",
	}, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Path != res.Path {
		t.Fatal("cache path changed")
	}
}

func TestFetchRejectsBadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.HTTP = srv.Client()
	dir := t.TempDir()
	_, err := c.Fetch(manifest.Pin{
		URL:       srv.URL + "/x.exe",
		SHA256:    strings.Repeat("0", 64),
		AssetName: "x.exe",
	}, dir, nil)
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("got %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.exe"))
	if len(matches) != 0 {
		t.Fatalf("left files %v", matches)
	}
}

func TestFetchResume(t *testing.T) {
	payload := []byte("ABCDEFGHIJKLMNOP")
	sum := sha256.Sum256(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rng := r.Header.Get("Range")
		if strings.HasPrefix(rng, "bytes=") {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 8-15/%d", len(payload)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(payload[8:])
			return
		}
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.HTTP = srv.Client()
	dir := t.TempDir()
	partial := filepath.Join(dir, "blob.bin.partial")
	if err := os.WriteFile(partial, payload[:8], 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := c.Fetch(manifest.Pin{
		URL:       srv.URL + "/blob.bin",
		SHA256:    hex.EncodeToString(sum[:]),
		Size:      int64(len(payload)),
		AssetName: "blob.bin",
	}, dir, func(got, total int64) {})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(res.Path)
	if string(got) != string(payload) {
		t.Fatalf("got %q", got)
	}
}

func TestRefuseNonHTTPS(t *testing.T) {
	c := New()
	_, err := c.Fetch(manifest.Pin{URL: "http://example.com/x", AssetName: "x"}, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected refuse")
	}
}
