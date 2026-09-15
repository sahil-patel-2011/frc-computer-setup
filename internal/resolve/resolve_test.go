package resolve

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
)

func TestMatchAssetGitNotPortable(t *testing.T) {
	rel := &GitHubRelease{
		TagName: "v2.55.0.windows.5",
		Assets: []GitHubAsset{
			{Name: "PortableGit-2.55.0.5-64-bit.7z.exe"},
			{Name: "Git-2.55.0.5-64-bit.exe", Digest: "sha256:" + strings.Repeat("a", 64), BrowserDownloadURL: "https://example.com/Git-2.55.0.5-64-bit.exe", Size: 10},
			{Name: "Git-2.55.0.5-64-bit.tar.bz2"},
		},
	}
	a, err := MatchAsset(rel, "Git-*-64-bit.exe")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "Git-2.55.0.5-64-bit.exe" {
		t.Fatalf("got %s", a.Name)
	}
}

func TestMatchAssetAmbiguous(t *testing.T) {
	rel := &GitHubRelease{
		TagName: "v1",
		Assets: []GitHubAsset{
			{Name: "Tool-win.exe"},
			{Name: "Tool-windows.exe"},
		},
	}
	if _, err := MatchAsset(rel, "Tool-win*.exe"); err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestPinFromGitHubRequiresDigest(t *testing.T) {
	rel := &GitHubRelease{
		TagName: "v1",
		HTMLURL: "https://github.com/x/y/releases/tag/v1",
		Assets: []GitHubAsset{
			{Name: "app.exe", BrowserDownloadURL: "https://example.com/app.exe", Size: 3},
		},
	}
	if _, err := PinFromGitHub(rel, "app.exe", time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("expected missing sha256")
	}
}

func TestWPILibNotes(t *testing.T) {
	body := `
## Downloads

- [Windows](https://packages.wpilib.workers.dev/installer/v2026.2.1/Win64/WPILib_Windows-2026.2.1.iso) (2.4 GB)

### SHA256 Hashes
` + "```\n" + `6dd86b714c41127c9ef7683b398dc21423cba7146e6868a97fefbd65a14429cb  Win64/WPILib_Windows-2026.2.1.iso
` + "```\n"
	rel := &GitHubRelease{TagName: "v2026.2.1", HTMLURL: "https://github.com/wpilibsuite/allwpilib/releases/tag/v2026.2.1", Body: body}
	pin, err := PinFromWPILibNotes(rel, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if pin.URL != "https://packages.wpilib.workers.dev/installer/v2026.2.1/Win64/WPILib_Windows-2026.2.1.iso" {
		t.Fatalf("url %s", pin.URL)
	}
	if pin.SHA256 != "6dd86b714c41127c9ef7683b398dc21423cba7146e6868a97fefbd65a14429cb" {
		t.Fatalf("sha %s", pin.SHA256)
	}
}

func TestWPILibNotesMissingURLIsVendor(t *testing.T) {
	rel := &GitHubRelease{TagName: "v2099.0.0", Body: "no downloads here"}
	if _, err := PinFromWPILibNotes(rel, time.Now()); err == nil {
		t.Fatal("expected error")
	}
}

func TestSHA256FromPipeTable(t *testing.T) {
	body := "Filename | SHA-256\n-------- | -------\nGit-2.55.0.5-64-bit.exe | d065a4e23c3d9a6b5073d609b5be0830227ec3ca053c083ba385061ddfaf94c6\n"
	got, ok := SHA256FromBody(body, "Git-2.55.0.5-64-bit.exe")
	if !ok || got != "d065a4e23c3d9a6b5073d609b5be0830227ec3ca053c083ba385061ddfaf94c6" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRefreshGitHubAndFallback(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name":"v9.0.0",
			"html_url":"https://github.com/o/r/releases/tag/v9.0.0",
			"prerelease":false,
			"assets":[{"name":"App-setup.exe","size":12,"browser_download_url":"http://HOST/App-setup.exe","digest":"sha256:` + sha + `"}]
		}`))
	})
	mux.HandleFunc("/App-setup.exe", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Content-Length", "12")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte("hello world!"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewClient("")
	client.HTTP = srv.Client()
	client.BaseAPI = srv.URL
	client.Now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

	rel, err := client.Latest("o", "r")
	if err != nil {
		t.Fatal(err)
	}
	rel.Assets[0].BrowserDownloadURL = srv.URL + "/App-setup.exe"
	pin, err := PinFromGitHub(rel, "App-setup.exe", client.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.FillHeaders(&pin); err != nil {
		t.Fatal(err)
	}
	if pin.ETag != `"abc"` || pin.Size != 12 {
		t.Fatalf("headers %+v", pin)
	}

	tool := manifest.Tool{ID: "x", Kind: "download", Source: &manifest.Source{Type: "nope"}}
	res := client.RefreshTool(tool)
	if !res.VendorFallback {
		t.Fatal("unknown source should vendor-fallback, not invent")
	}
}

func TestPrereleaseRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v2027.0.0-alpha","prerelease":true}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient("")
	c.HTTP = srv.Client()
	c.BaseAPI = srv.URL
	if _, err := c.Latest("a", "b"); err == nil {
		t.Fatal("expected prerelease error")
	}
}

func TestApplyVendorFallbackClearsPin(t *testing.T) {
	tool := manifest.Tool{
		ID: "ni", Kind: "download", DocsURL: "https://docs.example",
		Pinned: &manifest.Pin{URL: "https://example.com/x", SHA256: strings.Repeat("0", 64)},
	}
	Apply(&tool, Result{VendorFallback: true, Reason: "no public url"})
	if tool.Kind != "vendor_page" || tool.Pinned != nil {
		t.Fatalf("%+v", tool)
	}
	if tool.VendorURL != "https://docs.example" {
		t.Fatalf("vendor %s", tool.VendorURL)
	}
}

func TestCatalogSourceTypes(t *testing.T) {
	path := filepath.Join("..", "manifest", "tools.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := manifest.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	git, _ := c.Tool("git")
	if git.Source.Type != "github_release" {
		t.Fatal(git.Source.Type)
	}
	wpilib, _ := c.Tool("wpilib")
	if wpilib.Source.Type != "wpilib_github_notes" {
		t.Fatal(wpilib.Source.Type)
	}
}
