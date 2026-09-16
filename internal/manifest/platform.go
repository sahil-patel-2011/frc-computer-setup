package manifest

import (
	"fmt"
	"strings"
)

func NormalizeArch(goarch string) string {
	switch goarch {
	case "x86_64", "x64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return goarch
	}
}

func (t Tool) PinFor(goos, goarch string) *Pin {
	goarch = NormalizeArch(goarch)
	if t.Pins != nil {
		if p, ok := t.Pins[goos+"-"+goarch]; ok {
			cp := p
			return &cp
		}
		if p, ok := t.Pins[goos]; ok {
			cp := p
			return &cp
		}
		// Pins exist but not for this OS/arch — do not fall back to another platform.
		return nil
	}
	if t.Pinned != nil && goos == "windows" && goarch == "amd64" {
		cp := *t.Pinned
		return &cp
	}
	return nil
}

func (t Tool) InstallFor(goos, goarch string) *Install {
	goarch = NormalizeArch(goarch)
	if t.Installs != nil {
		if i, ok := t.Installs[goos+"-"+goarch]; ok {
			cp := i
			return &cp
		}
		if i, ok := t.Installs[goos]; ok {
			cp := i
			return &cp
		}
		return nil
	}
	if t.Install != nil {
		cp := *t.Install
		return &cp
	}
	return nil
}

func (t Tool) VendorURLFor(goos string) string {
	if t.VendorByOS != nil {
		if u, ok := t.VendorByOS[goos]; ok && u != "" {
			return u
		}
	}
	return t.VendorURL
}

func (t Tool) AvailableOn(goos string) bool {
	if t.WindowsOnly && goos != "windows" {
		return false
	}
	if len(t.Platforms) == 0 {
		return true
	}
	for _, p := range t.Platforms {
		if p == goos {
			return true
		}
	}
	return false
}

func (t Tool) EffectiveKind(goos, goarch string) string {
	if !t.AvailableOn(goos) {
		if t.WindowsOnly {
			return "windows_only"
		}
		return "unavailable"
	}
	if t.Kind == "git_clone" {
		return "git_clone"
	}
	if t.PinFor(goos, goarch) != nil {
		return "download"
	}
	if t.VendorURLFor(goos) != "" || t.Kind == "vendor_page" {
		return "vendor_page"
	}
	return "unavailable"
}

func (t Tool) Resolve(goos, goarch string) Tool {
	out := t
	kind := t.EffectiveKind(goos, goarch)
	out.Kind = kind
	if pin := t.PinFor(goos, goarch); pin != nil {
		out.Pinned = pin
		out.Kind = "download"
	} else {
		out.Pinned = nil
	}
	if inst := t.InstallFor(goos, goarch); inst != nil {
		out.Install = inst
	}
	if u := t.VendorURLFor(goos); u != "" {
		out.VendorURL = u
	}
	return out
}

func (t Tool) AllPins() []Pin {
	var out []Pin
	seen := map[string]struct{}{}
	add := func(p Pin) {
		key := p.URL
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	if t.Pinned != nil {
		add(*t.Pinned)
	}
	for _, p := range t.Pins {
		add(p)
	}
	return out
}

func validatePin(id string, p Pin) error {
	if p.URL == "" {
		return fmt.Errorf("tool %s pin missing url — use vendor_page instead of inventing one", id)
	}
	if p.SHA256 == "" && p.ETag == "" {
		return fmt.Errorf("tool %s pin has neither sha256 nor etag", id)
	}
	if !strings.HasPrefix(p.URL, "https://") {
		return fmt.Errorf("tool %s url must be https", id)
	}
	return nil
}
