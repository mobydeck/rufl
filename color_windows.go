//go:build windows
// +build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVirtualTerminalProcessing enables ANSI color support on Windows
func enableVirtualTerminalProcessing() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32

	err := windows.GetConsoleMode(stdout, &mode)
	if err != nil {
		colorSupported = false
		return
	}

	// Enable ENABLE_VIRTUAL_TERMINAL_PROCESSING
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	err = windows.SetConsoleMode(stdout, mode)
	if err != nil {
		colorSupported = false
		return
	}

	colorSupported = true
}

// isTerminal checks if the file descriptor is a terminal
func isTerminal(fd uintptr) bool {
	handle := windows.Handle(fd)
	var mode uint32
	err := windows.GetConsoleMode(handle, &mode)
	return err == nil
}
