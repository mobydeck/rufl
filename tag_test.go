package main

import (
	"reflect"
	"testing"
)

// TestProcessCommandsWithTags tests the processCommands function with various tag formats
func TestProcessCommandsWithTags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		tagFlags []string
		want     []CommandInfo
	}{
		{
			name: "Basic commands without tags",
			args: []string{"echo hello", "echo world"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "1", Index: 0},
				{Command: "echo world", Tag: "2", Index: 1},
			},
		},
		{
			name:     "Commands with -t flag only",
			args:     []string{"echo hello", "echo world"},
			tagFlags: []string{"greeting:echo hello", "farewell:echo world"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "greeting", Index: 0},
				{Command: "echo world", Tag: "farewell", Index: 1},
			},
		},
		{
			name: "Commands with + syntax only",
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
		{
			name:     "Invalid -t flag format",
			args:     []string{"echo hello", "echo world"},
			tagFlags: []string{"invalid-format"},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "1", Index: 0},
				{Command: "echo world", Tag: "2", Index: 1},
			},
		},
		{
			name: "Complex commands with + syntax",
			args: []string{
				"+complex:echo hello | grep hello",
				"+pipe:cat file.txt | grep pattern",
				"+redirect:echo hello > file.txt",
			},
			want: []CommandInfo{
				{Command: "echo hello | grep hello", Tag: "complex", Index: 0},
				{Command: "cat file.txt | grep pattern", Tag: "pipe", Index: 1},
				{Command: "echo hello > file.txt", Tag: "redirect", Index: 2},
			},
		},
		{
			name: "Commands with special characters in tags",
			args: []string{
				"+tag-with-dash:echo hello",
				"+tag_with_underscore:echo world",
			},
			want: []CommandInfo{
				{Command: "echo hello", Tag: "tag-with-dash", Index: 0},
				{Command: "echo world", Tag: "tag_with_underscore", Index: 1},
			},
		},
		{
			name: "Commands with duplicate tags",
			args: []string{
				"+same:echo first",
				"+same:echo second",
			},
			want: []CommandInfo{
				{Command: "echo first", Tag: "same", Index: 0},
				{Command: "echo second", Tag: "same", Index: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			tags = tt.tagFlags

			got := processCommands(tt.args)

			// Compare results
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processCommands() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTagPriority tests that tag priority is handled correctly
func TestTagPriority(t *testing.T) {
	// Test that when a command appears both as a positional argument and in a tag,
	// the tag is applied to the command in its original position

	// Reset global variables
	tags = []string{"greeting:echo hello"}

	args := []string{"echo hello", "echo world"}
	want := []CommandInfo{
		{Command: "echo hello", Tag: "greeting", Index: 0},
		{Command: "echo world", Tag: "2", Index: 1},
	}

	got := processCommands(args)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("processCommands() = %v, want %v", got, want)
	}

	// Test that when a command appears in both -t flag and + syntax,
	// both tags are applied to their respective commands

	// Reset global variables
	tags = []string{"greeting:echo hello"}

	args = []string{"+farewell:echo world"}
	want = []CommandInfo{
		{Command: "echo hello", Tag: "greeting", Index: 0},
		{Command: "echo world", Tag: "farewell", Index: 1},
	}

	got = processCommands(args)

	// The order of commands might vary depending on implementation details
	// So we'll check that both commands are present with the correct tags
	if len(got) != 2 {
		t.Errorf("processCommands() returned %d commands, want 2", len(got))
	} else {
		// Check that both commands are present with the correct tags
		foundGreeting := false
		foundFarewell := false

		for _, cmd := range got {
			if cmd.Command == "echo hello" && cmd.Tag == "greeting" {
				foundGreeting = true
			}
			if cmd.Command == "echo world" && cmd.Tag == "farewell" {
				foundFarewell = true
			}
		}

		if !foundGreeting {
			t.Errorf("processCommands() did not return command 'echo hello' with tag 'greeting'")
		}
		if !foundFarewell {
			t.Errorf("processCommands() did not return command 'echo world' with tag 'farewell'")
		}
	}
}
