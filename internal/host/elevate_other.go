//go:build !windows

package host

func (h *Real) ensureElevated() error {
	return nil
}

func RunInstallHelper(_ []string) error {
	return nil
}
