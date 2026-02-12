// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdash

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	StatusActive     = "active"
	StatusNeedsInput = "needs-input"
	StatusIdle       = "idle"
	StatusExited     = "exited"

	// How long after last output before transitioning to idle
	IdleTimeout = 10 * time.Second

	// Minimum time between status change callbacks to avoid rapid flapping
	DebounceInterval = 500 * time.Millisecond

	// Max bytes to keep in the line buffer
	MaxLineBufferSize = 4096
)

// Patterns that indicate Claude is waiting for user input.
// These match the end of terminal output when Claude's prompt is displayed.
var promptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`❯\s*$`),                       // Claude main prompt
	regexp.MustCompile(`>\s*$`),                        // Continuation prompt
	regexp.MustCompile(`\$\s*$`),                       // Shell prompt (after Claude exits)
	regexp.MustCompile(`\?\s*\(?(yes|no|y\/n)\)?`),       // Yes/no confirmation prompt
	regexp.MustCompile(`Do you want to proceed`),       // Claude permission prompt
	regexp.MustCompile(`\(Y\)es.*\(N\)o`),              // Claude Y/N prompt
	regexp.MustCompile(`Press Enter to continue`),      // Continue prompt
	regexp.MustCompile(`\[Y/n\]`),                      // Standard Y/n prompt
	regexp.MustCompile(`waiting for (?:input|response)`), // Explicit waiting messages
}

// StatusChangeCallback is called when the Claude session status changes.
// oldStatus may be empty on first detection.
type StatusChangeCallback func(oldStatus, newStatus string)

// StatusDetector monitors terminal output from a Claude Code session
// and detects status transitions (active, needs-input, idle, exited).
type StatusDetector struct {
	mu            sync.Mutex
	lineBuffer    string
	currentStatus string
	lastOutputAt  time.Time
	lastChangeAt  time.Time
	idleTimer     *time.Timer
	callback      StatusChangeCallback
	stopped       bool
}

// NewStatusDetector creates a new detector that will call the callback
// whenever the Claude session status changes.
func NewStatusDetector(callback StatusChangeCallback) *StatusDetector {
	sd := &StatusDetector{
		currentStatus: StatusActive,
		lastOutputAt:  time.Now(),
		callback:      callback,
	}
	sd.startIdleTimer()
	return sd
}

func (sd *StatusDetector) startIdleTimer() {
	sd.idleTimer = time.AfterFunc(IdleTimeout, func() {
		sd.mu.Lock()
		defer sd.mu.Unlock()
		if sd.stopped {
			return
		}
		// Only transition to idle if we're currently active and haven't received output recently
		if sd.currentStatus == StatusActive && time.Since(sd.lastOutputAt) >= IdleTimeout {
			sd.setStatus(StatusIdle)
		}
	})
}

func (sd *StatusDetector) resetIdleTimer() {
	if sd.idleTimer != nil {
		sd.idleTimer.Stop()
	}
	if !sd.stopped {
		sd.idleTimer = time.AfterFunc(IdleTimeout, func() {
			sd.mu.Lock()
			defer sd.mu.Unlock()
			if sd.stopped {
				return
			}
			if sd.currentStatus == StatusActive && time.Since(sd.lastOutputAt) >= IdleTimeout {
				sd.setStatus(StatusIdle)
			}
		})
	}
}

// setStatus updates the status and fires the callback if changed.
// Must be called with mu held.
func (sd *StatusDetector) setStatus(newStatus string) {
	if newStatus == sd.currentStatus {
		return
	}
	// Debounce: don't change status too rapidly
	if time.Since(sd.lastChangeAt) < DebounceInterval {
		return
	}
	oldStatus := sd.currentStatus
	sd.currentStatus = newStatus
	sd.lastChangeAt = time.Now()
	if sd.callback != nil {
		// Fire callback outside the lock
		go sd.callback(oldStatus, newStatus)
	}
}

// ProcessOutput feeds terminal output data to the detector.
// Called from the PTY read loop with each chunk of data.
func (sd *StatusDetector) ProcessOutput(data []byte) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if sd.stopped {
		return
	}

	sd.lastOutputAt = time.Now()
	sd.resetIdleTimer()

	// Append to line buffer, keeping only the tail
	sd.lineBuffer += string(data)
	if len(sd.lineBuffer) > MaxLineBufferSize {
		sd.lineBuffer = sd.lineBuffer[len(sd.lineBuffer)-MaxLineBufferSize:]
	}

	// Get the last few lines for pattern matching
	lastLines := getLastLines(sd.lineBuffer, 3)
	stripped := stripAnsi(lastLines)

	// Check if output matches a prompt pattern (needs-input)
	if matchesPrompt(stripped) {
		sd.setStatus(StatusNeedsInput)
	} else {
		// We're receiving output, so Claude is active
		sd.setStatus(StatusActive)
	}
}

// SetExited marks the session as exited. Called when the process exits.
func (sd *StatusDetector) SetExited() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.setStatus(StatusExited)
}

// Stop cleans up the detector's resources.
func (sd *StatusDetector) Stop() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.stopped = true
	if sd.idleTimer != nil {
		sd.idleTimer.Stop()
	}
}

// GetStatus returns the current detected status.
func (sd *StatusDetector) GetStatus() string {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.currentStatus
}

// matchesPrompt checks if the text matches any known Claude prompt pattern.
func matchesPrompt(text string) bool {
	for _, pat := range promptPatterns {
		if pat.MatchString(text) {
			return true
		}
	}
	return false
}

// getLastLines returns the last n lines from the text.
func getLastLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= n {
		return text
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// stripAnsi removes ANSI escape codes from terminal output.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[[0-9;]*m`)

func stripAnsi(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}
