package engine

import (
	"regexp"
	"strings"
)

var versionToken = regexp.MustCompile(`\d+(?:\.\d+)+`)

func NormalizeVersion(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "v")
	if m := versionToken.FindString(s); m != "" {
		return m
	}
	return s
}

func VersionCurrent(installed, pin string) bool {
	a, b := NormalizeVersion(installed), NormalizeVersion(pin)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}
