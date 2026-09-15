package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/resolve"
)

func main() {
	catalog, err := manifest.Default()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client := resolve.NewClient(os.Getenv("GITHUB_TOKEN"))
	client.Now = time.Now
	changed := 0
	for i := range catalog.Tools {
		tool := &catalog.Tools[i]
		if tool.Kind == "vendor_page" && tool.Source == nil {
			continue
		}
		if tool.Source == nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "refresh %s …\n", tool.ID)
		result := client.RefreshTool(*tool)
		before := pinKey(tool)
		resolve.Apply(tool, result)
		after := pinKey(tool)
		if before != after {
			changed++
			fmt.Fprintf(os.Stderr, "  %s\n", after)
		}
		if result.VendorFallback {
			fmt.Fprintf(os.Stderr, "  vendor step: %s\n", result.Reason)
		}
	}
	catalog.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	out, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path := "internal/manifest/tools.json"
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d tools, %d changed)\n", path, len(catalog.Tools), changed)
}

func pinKey(t *manifest.Tool) string {
	if t.Kind == "vendor_page" || t.Pinned == nil {
		return t.ID + ":vendor"
	}
	return t.ID + ":" + t.Pinned.Version + ":" + t.Pinned.SHA256
}
