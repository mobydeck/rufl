//go:build !windows
// +build !windows

package main

import (
	"os"
)

// enableVirtualTerminalProcessing enables ANSI color support on Unix-like systems
func enableVirtualTerminalProcessing() {
	// On non-Windows platforms, assume color is supported
	// unless the terminal is not a TTY or NO_COLOR env var is set
	colorSupported = isTerminal(os.Stdout.Fd()) && os.Getenv("NO_COLOR") == ""
}

// isTerminal checks if the file descriptor is a terminal
func isTerminal(fd uintptr) bool {
	// For Unix-like systems, we could use isatty
	// but for simplicity, we'll just assume it's a terminal if it's a character device
	// A more complete solution would use golang.org/x/crypto/ssh/terminal
	// or golang.org/x/term package
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
