package manifest

import "testing"

func TestPinForLinuxWPILib(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	wpilib, ok := c.Tool("wpilib")
	if !ok {
		t.Fatal("missing wpilib")
	}
	pin := wpilib.PinFor("linux", "amd64")
	if pin == nil || pin.SHA256 != "c36591be0b5d1b753356543e0e672af9d91335fb26b5ffcba31cf05af829c656" {
		t.Fatalf("linux wpilib pin %+v", pin)
	}
	if pin.URL != "https://packages.wpilib.workers.dev/installer/v2026.2.1/Linux/WPILib_Linux-2026.2.1.tar.gz" {
		t.Fatalf("url %s", pin.URL)
	}
	mac := wpilib.PinFor("darwin", "arm64")
	if mac == nil || mac.AssetName != "WPILib_macOS-Arm64-2026.2.1.dmg" {
		t.Fatalf("mac pin %+v", mac)
	}
}

func TestNIWindowsOnly(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	ni, _ := c.Tool("ni-game-tools")
	if ni.Kind != "vendor_page" || ni.Pinned != nil {
		t.Fatalf("NI must be vendor_page with no pin: %+v", ni)
	}
	if ni.AvailableOn("linux") || ni.AvailableOn("darwin") {
		t.Fatal("NI must not be available off Windows")
	}
	if !ni.AvailableOn("windows") {
		t.Fatal("NI should be available on Windows")
	}
	if ni.EffectiveKind("linux", "amd64") != "windows_only" {
		t.Fatalf("kind %s", ni.EffectiveKind("linux", "amd64"))
	}
}

func TestGitLinuxIsVendorNotInvented(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	git, _ := c.Tool("git")
	if git.PinFor("linux", "amd64") != nil {
		t.Fatal("do not invent a Linux Git installer URL")
	}
	if git.EffectiveKind("linux", "amd64") != "vendor_page" {
		t.Fatalf("kind %s", git.EffectiveKind("linux", "amd64"))
	}
	if git.VendorURLFor("linux") != "https://git-scm.com/download/linux" {
		t.Fatal(git.VendorURLFor("linux"))
	}
}

func TestPinForDoesNotCrossArch(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	wpilib, _ := c.Tool("wpilib")
	if pin := wpilib.PinFor("windows", "arm64"); pin != nil {
		t.Fatalf("do not hand the Win64 ISO to Windows ARM: %+v", pin)
	}
	pp, _ := c.Tool("pathplanner")
	if pin := pp.PinFor("linux", "arm64"); pin != nil {
		t.Fatal("do not invent PathPlanner Linux ARM")
	}
}

func TestPhoenixLinuxUnavailableNotWindowsOnly(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	ph, _ := c.Tool("phoenix-tuner-x")
	if ph.EffectiveKind("linux", "amd64") != "unavailable" {
		t.Fatalf("kind %s", ph.EffectiveKind("linux", "amd64"))
	}
	if ph.EffectiveKind("darwin", "arm64") != "vendor_page" {
		t.Fatalf("mac kind %s", ph.EffectiveKind("darwin", "arm64"))
	}
}

func TestSilentWindowsArgs(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"git", "pathplanner", "advantagescope", "choreo", "elastic"} {
		tool, _ := c.Tool(id)
		inst := tool.InstallFor("windows", "amd64")
		if inst == nil || inst.Type != "exe" || len(inst.Args) == 0 {
			t.Fatalf("%s missing Windows silent args: %+v", id, inst)
		}
	}
	wpilib, _ := c.Tool("wpilib")
	if inst := wpilib.InstallFor("windows", "amd64"); inst == nil || inst.Type != "wpilib_iso" {
		t.Fatalf("wpilib windows %+v", wpilib.InstallFor("windows", "amd64"))
	}
	if inst := wpilib.InstallFor("darwin", "arm64"); inst == nil || inst.Type != "wpilib_dmg" {
		t.Fatalf("wpilib mac %+v", inst)
	}
	if inst := wpilib.InstallFor("linux", "amd64"); inst == nil || inst.Type != "wpilib_tarball" {
		t.Fatalf("wpilib linux %+v", inst)
	}
}
