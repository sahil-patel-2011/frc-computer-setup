package host

import "strings"

// HasSilentFlags reports whether the vendor installer can run unattended.
// Guessing a flag (and popping a GUI) is not allowed — callers skip instead.
func HasSilentFlags(args []string) bool {
	for _, a := range args {
		switch strings.ToUpper(strings.TrimSpace(a)) {
		case "/S", "/VERYSILENT", "/SILENT", "/QUIET", "/QN", "--FORCE", "-Y", "--YES":
			return true
		}
	}
	return false
}
