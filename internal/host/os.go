package host

import "runtime"

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func currentGOOS() string {
	return runtime.GOOS
}

func currentGOARCH() string {
	return runtime.GOARCH
}
