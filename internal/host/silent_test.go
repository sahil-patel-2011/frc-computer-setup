package host

import "testing"

func TestHasSilentFlags(t *testing.T) {
	if HasSilentFlags(nil) || HasSilentFlags([]string{"/NORESTART"}) {
		t.Fatal("norestart alone is not silent")
	}
	if !HasSilentFlags([]string{"/VERYSILENT", "/NORESTART"}) {
		t.Fatal("inno")
	}
	if !HasSilentFlags([]string{"/S"}) {
		t.Fatal("nsis")
	}
	if !HasSilentFlags([]string{"--force", "-y"}) {
		t.Fatal("wpilib cli")
	}
}
