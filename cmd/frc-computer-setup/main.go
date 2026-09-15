package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/sahil-patel-2011/frc-computer-setup/internal/download"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/engine"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/host"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/manifest"
	"github.com/sahil-patel-2011/frc-computer-setup/internal/wizard"
)

func main() {
	cli := flag.Bool("cli", false, "text walkthrough instead of the window")
	demo := flag.Bool("demo", false, "walk the wizard without downloading vendor installers")
	printManifest := flag.Bool("print-manifest", false, "print the pinned catalog and exit")
	toolsFlag := flag.String("tools", "", "comma-separated tool ids (cli only)")
	flag.Parse()

	catalog, err := manifest.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "manifest: %v\n", err)
		os.Exit(1)
	}

	if *printManifest {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(catalog)
		return
	}

	h := host.NewReal()
	if !*demo && runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "This installer is Windows-only (FRC Driver Station is Windows-only).")
		fmt.Fprintln(os.Stderr, "Use --demo to preview the walkthrough on this computer.")
		os.Exit(2)
	}

	dl := download.New()
	runner := &engine.Runner{Catalog: catalog, Host: h, DL: dl, Demo: *demo}
	runner.Emit = func(e engine.Event) {
		if *cli {
			if e.Total > 0 && e.Phase == engine.PhaseDownload {
				fmt.Printf("\r%s: %s  %d / %d bytes", e.Name, e.Message, e.Got, e.Total)
				if e.Got == e.Total {
					fmt.Println()
				}
				return
			}
			if e.Name != "" {
				fmt.Printf("%s — %s\n", e.Name, e.Message)
			} else if e.Message != "" {
				fmt.Println(e.Message)
			}
		}
	}

	if *cli {
		ids := engine.DefaultSelected(catalog)
		if *toolsFlag != "" {
			ids = splitCSV(*toolsFlag)
		}
		runCLI(runner, catalog, ids)
		return
	}

	srv := wizard.New(catalog, runner, h, *demo)
	url, err := srv.ListenAndServe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "wizard: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Setup window:", url)
	if err := wizard.LaunchInAppWindow(h, url); err != nil {
		fmt.Fprintf(os.Stderr, "open browser: %v\nopen %s yourself\n", err, url)
	}
	select {}
}

func runCLI(runner *engine.Runner, catalog *manifest.Catalog, ids []string) {
	in := bufio.NewReader(os.Stdin)
	for _, id := range ids {
		tool, ok := catalog.Tool(id)
		if !ok {
			fmt.Printf("Unknown tool %s — skipped.\n", id)
			continue
		}
		var ack chan string
		if tool.Kind == "vendor_page" {
			fmt.Printf("Official page: %s\nType done when finished, or skip:\n", tool.VendorURL)
			ack = make(chan string, 1)
			go func() {
				line, _ := in.ReadString('\n')
				line = strings.TrimSpace(strings.ToLower(line))
				if line == "s" || line == "skip" {
					ack <- "skip"
					return
				}
				ack <- "installed"
			}()
		}
		res := runner.RunTool(tool, ack)
		fmt.Printf("→ %s: %s\n\n", res.Tool.Name, res.Message)
	}
	fmt.Println("Walkthrough finished.")
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
