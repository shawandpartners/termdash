// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdash

import (
	"sync"
	"testing"
	"time"
)

func TestMatchesPrompt(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"claude main prompt", "❯ ", true},
		{"claude main prompt with trailing space", "❯  ", true},
		{"continuation prompt", "> ", true},
		{"shell prompt", "$ ", true},
		{"yn prompt", "? (yes/no) ", true},
		{"Y/N prompt", "(Y)es / (N)o", true},
		{"regular output", "Building project...", false},
		{"code output", "const foo = 42;", false},
		{"empty string", "", false},
		{"press enter", "Press Enter to continue", true},
		{"Y/n bracket", "[Y/n]", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPrompt(tt.input)
			if got != tt.want {
				t.Errorf("matchesPrompt(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no ansi", "hello world", "hello world"},
		{"color code", "\x1b[32mgreen\x1b[0m", "green"},
		{"cursor movement", "\x1b[1;1Hhello", "hello"},
		{"osc title", "\x1b]0;My Title\x07rest", "rest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAnsi(tt.input)
			if got != tt.want {
				t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetLastLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"single line", "hello", 3, "hello"},
		{"three lines get three", "a\nb\nc", 3, "a\nb\nc"},
		{"five lines get last three", "a\nb\nc\nd\ne", 3, "c\nd\ne"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLastLines(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("getLastLines(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestStatusDetectorTransitions(t *testing.T) {
	var mu sync.Mutex
	var transitions []string

	callback := func(oldStatus, newStatus string) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, oldStatus+"->"+newStatus)
	}

	sd := NewStatusDetector(callback)
	defer sd.Stop()

	// Initial status should be active
	if got := sd.GetStatus(); got != StatusActive {
		t.Errorf("initial status = %q, want %q", got, StatusActive)
	}

	// Feed some regular output — should stay active
	sd.ProcessOutput([]byte("Building project...\n"))
	time.Sleep(10 * time.Millisecond)
	if got := sd.GetStatus(); got != StatusActive {
		t.Errorf("after output status = %q, want %q", got, StatusActive)
	}

	// Wait for debounce to pass, then feed a prompt
	time.Sleep(DebounceInterval + 50*time.Millisecond)
	sd.ProcessOutput([]byte("\n❯ "))
	time.Sleep(50 * time.Millisecond)

	if got := sd.GetStatus(); got != StatusNeedsInput {
		t.Errorf("after prompt status = %q, want %q", got, StatusNeedsInput)
	}

	// Feed more output — should transition back to active
	time.Sleep(DebounceInterval + 50*time.Millisecond)
	sd.ProcessOutput([]byte("Analyzing code...\nRunning tests...\n"))
	time.Sleep(50 * time.Millisecond)

	if got := sd.GetStatus(); got != StatusActive {
		t.Errorf("after more output status = %q, want %q", got, StatusActive)
	}

	// Mark as exited
	time.Sleep(DebounceInterval + 50*time.Millisecond)
	sd.SetExited()
	time.Sleep(50 * time.Millisecond)

	if got := sd.GetStatus(); got != StatusExited {
		t.Errorf("after exit status = %q, want %q", got, StatusExited)
	}

	// Verify transitions happened
	mu.Lock()
	defer mu.Unlock()
	if len(transitions) < 2 {
		t.Errorf("expected at least 2 transitions, got %d: %v", len(transitions), transitions)
	}
}
