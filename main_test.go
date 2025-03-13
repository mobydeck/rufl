package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestProcessCommands(t *testing.T) {
	// Reset global variables before each test
	tags = []string{}

	tests := []struct {
		name     string
		args     []string
		tagFlags []string
		want     []CommandInfo
	}{
		{
			name: "Basic commands",
			args: []string{"echo hello", "echo world"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "1", Index: 0},
				{Command: "echo world", Tag: "2", Index: 1},
			},
		},
		{
			name:     "Commands with -t flag",
			args:     []string{"echo hello", "echo world"},
			tagFlags: []string{"greeting:echo hello", "farewell:echo world"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "greeting", Index: 0},
				{Command: "echo world", Tag: "farewell", Index: 1},
			},
		},
		{
			name: "Commands with + syntax",
			args: []string{"+greeting:echo hello", "+farewell:echo world"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "greeting", Index: 0},
				{Command: "echo world", Tag: "farewell", Index: 1},
			},
		},
		{
			name: "Mixed regular and + syntax",
			args: []string{"echo hello", "+farewell:echo world"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "1", Index: 0},
				{Command: "echo world", Tag: "farewell", Index: 1},
			},
		},
		{
			name:     "Mixed regular, + syntax, and -t flag",
			args:     []string{"echo hello", "+farewell:echo world"},
			tagFlags: []string{"greeting:echo hello"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "greeting", Index: 0},
				{Command: "echo world", Tag: "farewell", Index: 1},
			},
		},
		{
			name: "Invalid + syntax",
			args: []string{"+invalid-format", "echo hello"},
			want: []CommandInfo{
				{Command: "+invalid-format", Tag: "1", Index: 0},
				{Command: "echo hello", Tag: "2", Index: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up tag flags
			tags = tt.tagFlags

			got := processCommands(tt.args)

			// Compare results
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processCommands() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsShell(t *testing.T) {
	// Reset global variables before each test
	forceShell = false
	envVars = []string{}

	tests := []struct {
		name    string
		command string
		envVars []string
		force   bool
		want    bool
	}{
		{
			name:    "Simple command",
			command: "echo hello",
			want:    false,
		},
		{
			name:    "Command with pipe",
			command: "echo hello | grep hello",
			want:    true,
		},
		{
			name:    "Command with redirection",
			command: "echo hello > file.txt",
			want:    true,
		},
		{
			name:    "Command with environment variable",
			command: "echo $HOME",
			want:    true,
		},
		{
			name:    "Command with glob",
			command: "ls *.txt",
			want:    true,
		},
		{
			name:    "Command with quotes",
			command: "echo \"hello world\"",
			want:    true,
		},
		{
			name:    "Command with &&",
			command: "echo hello && echo world",
			want:    true,
		},
		{
			name:    "Force shell",
			command: "echo hello",
			force:   true,
			want:    true,
		},
		{
			name:    "With environment variables set",
			command: "echo hello",
			envVars: []string{"VAR=value"},
			want:    true,
		},
		// Additional test cases for better coverage
		{
			name:    "Command with semicolon",
			command: "echo hello; echo world",
			want:    true,
		},
		{
			name:    "Command with ||",
			command: "false || echo fallback",
			want:    true,
		},
		{
			name:    "Command with input redirection",
			command: "cat < file.txt",
			want:    true,
		},
		{
			name:    "Command with question mark glob",
			command: "ls file?.txt",
			want:    true,
		},
		{
			name:    "Command with bracket glob",
			command: "ls file[123].txt",
			want:    true,
		},
		{
			name:    "Command with backtick",
			command: "echo `date`",
			want:    true,
		},
		{
			name:    "Command with single quotes",
			command: "echo 'hello world'",
			want:    true,
		},
		{
			name:    "Command with hash",
			command: "echo # comment",
			want:    true,
		},
		{
			name:    "Command with tilde",
			command: "ls ~/Documents",
			want:    true,
		},
		{
			name:    "Command with equals",
			command: "FOO=bar echo hello",
			want:    true,
		},
		{
			name:    "Command with percent",
			command: "echo %PATH%",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up global variables
			forceShell = tt.force
			envVars = tt.envVars

			got := needsShell(tt.command)
			if got != tt.want {
				t.Errorf("needsShell() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExecuteCommand is an integration test that actually runs commands
func TestExecuteCommand(t *testing.T) {
	// Skip if running in CI environment
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "rufl-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Reset global variables
	noColor = true // Disable color for testing
	colorSupported = false

	// Test a simple command
	cmdInfo := CommandInfo{
		Command: "echo test output",
		Tag:     "test",
		Index:   0,
	}

	executeCommand(cmdInfo)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	output := buf.String()

	// Check if the output contains the expected text
	if !strings.Contains(output, "test output") {
		t.Errorf("executeCommand() output = %v, want to contain 'test output'", output)
	}

	// Check if the output contains the tag
	if !strings.Contains(output, "[test:") {
		t.Errorf("executeCommand() output = %v, want to contain '[test:'", output)
	}
}

// TestRunCommands tests both parallel and sequential execution
func TestRunCommands(t *testing.T) {
	// Skip if running in CI environment
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Reset global variables
	noColor = true // Disable color for testing
	colorSupported = false

	commands := []CommandInfo{
		{Command: "echo first", Tag: "1", Index: 0},
		{Command: "echo second", Tag: "2", Index: 1},
	}

	// Test sequential execution
	runCommands(commands, false)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	output := buf.String()

	// Check if the output contains both commands' output
	if !strings.Contains(output, "first") || !strings.Contains(output, "second") {
		t.Errorf("runCommands() output = %v, want to contain 'first' and 'second'", output)
	}

	// Now test parallel execution
	r, w, _ = os.Pipe()
	os.Stdout = w

	runCommands(commands, true)

	w.Close()
	os.Stdout = oldStdout

	buf.Reset()
	_, err = buf.ReadFrom(r)
	if err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	output = buf.String()

	// Check if the output contains both commands' output
	if !strings.Contains(output, "first") || !strings.Contains(output, "second") {
		t.Errorf("runCommands() output = %v, want to contain 'first' and 'second'", output)
	}
}

// TestColorSupport tests the color support functions
func TestColorSupport(t *testing.T) {
	// This is mostly a smoke test since we can't easily test the actual color output
	oldColorSupported := colorSupported
	oldNoColor := noColor

	// Test with color disabled
	noColor = true
	colorSupported = true

	// Capture stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	printColoredMessage("Test message", colorRed)

	// Close the write end of the pipe to flush the buffers
	w.Close()

	// Read the captured output
	var outBuf bytes.Buffer
	io.Copy(&outBuf, r)

	os.Stdout = oldStdout

	output := outBuf.String()
	if !strings.Contains(output, "Test message") {
		t.Errorf("printColoredMessage() output = %v, want to contain 'Test message'", output)
	}
	if strings.Contains(output, colorRed) {
		t.Errorf("printColoredMessage() output = %v, should not contain color codes when noColor=true", output)
	}

	// Test with color enabled
	r, w, _ = os.Pipe()
	os.Stdout = w

	noColor = false
	colorSupported = true

	printColoredMessage("Test message", colorRed)

	// Close the write end of the pipe to flush the buffers
	w.Close()

	// Read the captured output
	outBuf.Reset()
	io.Copy(&outBuf, r)

	os.Stdout = oldStdout

	output = outBuf.String()
	if !strings.Contains(output, "Test message") {
		t.Errorf("printColoredMessage() output = %v, want to contain 'Test message'", output)
	}

	// Restore global variables
	colorSupported = oldColorSupported
	noColor = oldNoColor
}

// TestMain is a helper function to run the tests
func TestMain(m *testing.M) {
	// Skip tests that require command execution if we're not on a supported platform
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		fmt.Println("Skipping tests that require command execution on unsupported platform")
		os.Exit(0)
	}

	// Check if we have the required commands
	_, err := exec.LookPath("echo")
	if err != nil {
		fmt.Println("'echo' command not found, skipping tests that require command execution")
		os.Exit(0)
	}

	// Run the tests
	os.Exit(m.Run())
}
