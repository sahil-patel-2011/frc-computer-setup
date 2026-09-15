package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

const userAgent = "frc-computer-setup (https://github.com/sahil-patel-2011/frc-computer-setup)"

type ProgressFunc func(got, total int64)

type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 0}}
}

type Result struct {
	Path   string
	SHA256 string
	Size   int64
	ETag   string
}

func (c *Client) Fetch(pin manifest.Pin, destDir string, progress ProgressFunc) (Result, error) {
	if !strings.HasPrefix(pin.URL, "https://") && !strings.HasPrefix(pin.URL, "http://127.0.0.1") && !strings.HasPrefix(pin.URL, "http://localhost") && !strings.HasPrefix(pin.URL, "http://[::1]") {
		// tests use httptest (http://127.0.0.1). production pins are https-only via manifest.Validate.
		if !strings.HasPrefix(pin.URL, "http://127.0.0.1") {
			return Result{}, fmt.Errorf("refusing non-https URL")
		}
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return Result{}, err
	}
	name := pin.AssetName
	if name == "" {
		name = filepath.Base(pin.URL)
	}
	final := filepath.Join(destDir, name)
	partial := final + ".partial"

	if st, err := os.Stat(final); err == nil && st.Size() > 0 {
		sum, err := HashFile(final)
		if err == nil && pin.SHA256 != "" && strings.EqualFold(sum, pin.SHA256) {
			if progress != nil {
				progress(st.Size(), st.Size())
			}
			return Result{Path: final, SHA256: strings.ToLower(sum), Size: st.Size(), ETag: pin.ETag}, nil
		}
		if pin.SHA256 == "" && pin.Size > 0 && st.Size() == pin.Size {
			sum, err := HashFile(final)
			if err == nil {
				return Result{Path: final, SHA256: sum, Size: st.Size(), ETag: pin.ETag}, nil
			}
		}
	}

	req, err := http.NewRequest(http.MethodGet, pin.URL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	if pin.ETag != "" {
		req.Header.Set("If-None-Match", pin.ETag)
	}
	var have int64
	if st, err := os.Stat(partial); err == nil {
		have = st.Size()
		if have > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
		}
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 0}
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		sum, err := HashFile(final)
		if err != nil {
			return Result{}, fmt.Errorf("etag hit but cache missing: %w", err)
		}
		if pin.SHA256 != "" && !strings.EqualFold(sum, pin.SHA256) {
			_ = os.Remove(final)
			return Result{}, fmt.Errorf("cached file failed sha256")
		}
		st, _ := os.Stat(final)
		return Result{Path: final, SHA256: strings.ToLower(sum), Size: st.Size(), ETag: pin.ETag}, nil
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		return Result{}, fmt.Errorf("download HTTP %d for %s", res.StatusCode, pin.URL)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if res.StatusCode == http.StatusPartialContent && have > 0 {
		flags |= os.O_APPEND
	} else {
		have = 0
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(partial, flags, 0o644)
	if err != nil {
		return Result{}, err
	}

	total := res.ContentLength
	if total > 0 {
		total += have
	}
	if pin.Size > 0 {
		total = pin.Size
	}
	hash := sha256.New()
	if have > 0 {
		rf, err := os.Open(partial)
		if err != nil {
			f.Close()
			return Result{}, err
		}
		if _, err := io.Copy(hash, io.LimitReader(rf, have)); err != nil {
			rf.Close()
			f.Close()
			return Result{}, err
		}
		rf.Close()
	}

	buf := make([]byte, 256*1024)
	got := have
	for {
		n, readErr := res.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return Result{}, err
			}
			_, _ = hash.Write(buf[:n])
			got += int64(n)
			if progress != nil {
				progress(got, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			return Result{}, readErr
		}
	}
	if err := f.Close(); err != nil {
		return Result{}, err
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	if pin.SHA256 != "" && !strings.EqualFold(sum, pin.SHA256) {
		_ = os.Remove(partial)
		return Result{}, fmt.Errorf("sha256 mismatch for %s: got %s want %s", name, sum, pin.SHA256)
	}
	if pin.SHA256 == "" && pin.ETag != "" {
		gotETag := strings.TrimSpace(res.Header.Get("ETag"))
		if gotETag != "" && gotETag != pin.ETag {
			_ = os.Remove(partial)
			return Result{}, fmt.Errorf("etag mismatch for %s", name)
		}
	}
	if pin.Size > 0 && got != pin.Size {
		_ = os.Remove(partial)
		return Result{}, fmt.Errorf("size mismatch for %s: got %d want %d", name, got, pin.Size)
	}
	if err := os.Rename(partial, final); err != nil {
		return Result{}, err
	}
	etag := strings.TrimSpace(res.Header.Get("ETag"))
	if etag == "" {
		etag = pin.ETag
	}
	return Result{Path: final, SHA256: sum, Size: got, ETag: etag}, nil
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "FRC-Computer-Setup", "cache")
	return dir, os.MkdirAll(dir, 0o755)
}
