package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestProcessOutputPreservesColors tests that the processOutput function preserves ANSI color codes
func TestProcessOutputPreservesColors(t *testing.T) {
	// Save original stdout and color settings
	oldStdout := os.Stdout
	oldNoColor := noColor
	oldColorSupported := colorSupported

	// Enable color support for this test
	noColor = false
	colorSupported = true

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a reader with colored text
	coloredText := "\033[31mRed text\033[0m \033[32mGreen text\033[0m"
	reader := strings.NewReader(coloredText)

	// Process the output
	processOutput(reader, "test", "out", colorGreen)

	// Close the write end of the pipe to flush the buffers
	w.Close()

	// Read the captured output
	var outBuf bytes.Buffer
	io.Copy(&outBuf, r)

	// Restore stdout and color settings
	os.Stdout = oldStdout
	noColor = oldNoColor
	colorSupported = oldColorSupported

	// Get the captured output
	output := outBuf.String()

	// Check that the output contains the color codes from the input
	if !strings.Contains(output, "\033[31m") || !strings.Contains(output, "\033[32m") {
		t.Errorf("processOutput() did not preserve color codes, output = %q", output)
	}

	// Check that the prefix is colored with the specified color
	if !strings.Contains(output, colorGreen+"[test]") {
		t.Errorf("processOutput() did not color the prefix, output = %q", output)
	}
}

// TestProcessOutputWithNoColor tests that the processOutput function doesn't use colors when noColor is true
func TestProcessOutputWithNoColor(t *testing.T) {
	// Save original stdout and color settings
	oldStdout := os.Stdout
	oldNoColor := noColor
	oldColorSupported := colorSupported

	// Disable color support for this test
	noColor = true
	colorSupported = true

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a reader with colored text
	coloredText := "\033[31mRed text\033[0m \033[32mGreen text\033[0m"
	reader := strings.NewReader(coloredText)

	// Process the output
	processOutput(reader, "test", "out", colorGreen)

	// Close the write end of the pipe to flush the buffers
	w.Close()

	// Read the captured output
	var outBuf bytes.Buffer
	io.Copy(&outBuf, r)

	// Restore stdout and color settings
	os.Stdout = oldStdout
	noColor = oldNoColor
	colorSupported = oldColorSupported

	// Get the captured output
	output := outBuf.String()

	// Check that the output doesn't contain the color code for the prefix
	// We're checking that the prefix "[test:out]" is not colored with colorGreen
	if strings.Contains(output, colorGreen+"[test:out]") {
		t.Errorf("processOutput() used colors for prefix when noColor=true, output = %q", output)
	}

	// The input color codes should still be in the output as text
	// This is expected behavior - we don't strip color codes from the content
	if !strings.Contains(output, "\033[31m") {
		t.Errorf("processOutput() should preserve color codes in content, output = %q", output)
	}
}

// TestPrintColoredMessage tests the printColoredMessage function
func TestPrintColoredMessage(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		color          string
		noColor        bool
		colorSupported bool
		wantColor      bool
	}{
		{
			name:           "With color enabled",
			message:        "Test message",
			color:          colorRed,
			noColor:        false,
			colorSupported: true,
			wantColor:      true,
		},
		{
			name:           "With noColor flag",
			message:        "Test message",
			color:          colorRed,
			noColor:        true,
			colorSupported: true,
			wantColor:      false,
		},
		{
			name:           "With color not supported",
			message:        "Test message",
			color:          colorRed,
			noColor:        false,
			colorSupported: false,
			wantColor:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original settings
			oldStdout := os.Stdout
			oldNoColor := noColor
			oldColorSupported := colorSupported

			// Apply test settings
			noColor = tt.noColor
			colorSupported = tt.colorSupported

			// Capture output
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call the function
			printColoredMessage(tt.message, tt.color)

			// Close the write end of the pipe to flush the buffers
			w.Close()

			// Read the captured output
			var outBuf bytes.Buffer
			io.Copy(&outBuf, r)

			// Restore settings
			os.Stdout = oldStdout
			noColor = oldNoColor
			colorSupported = oldColorSupported

			// Check the output
			output := outBuf.String()

			// The message should always be present
			if !strings.Contains(output, tt.message) {
				t.Errorf("printColoredMessage() output = %q, want to contain %q", output, tt.message)
			}

			// Check if color is present as expected
			hasColor := strings.Contains(output, tt.color)
			if hasColor != tt.wantColor {
				if tt.wantColor {
					t.Errorf("printColoredMessage() output = %q, want to contain color %q", output, tt.color)
				} else {
					t.Errorf("printColoredMessage() output = %q, should not contain color %q", output, tt.color)
				}
			}
		})
	}
}
