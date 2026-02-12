// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdash

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// Minimum bytes between transcript flushes
	TranscriptFlushThreshold = 1024

	// Maximum time between flushes
	TranscriptFlushInterval = 5 * time.Second
)

// TranscriptEntry represents a single entry in the transcript log.
type TranscriptEntry struct {
	Timestamp int64  `json:"ts"`
	Type      string `json:"type"` // "output" or "input"
	Text      string `json:"text"`
}

// TranscriptFlushFunc is called when the transcript buffer should be persisted.
// The data is JSONL-formatted (one JSON object per line).
type TranscriptFlushFunc func(data []byte)

// TranscriptRecorder records cleaned terminal I/O for a Claude session.
// It strips ANSI codes, deduplicates animation noise, and batches writes.
type TranscriptRecorder struct {
	mu          sync.Mutex
	buffer      []TranscriptEntry
	bufferBytes int
	flushFn     TranscriptFlushFunc
	flushTimer  *time.Timer
	stopped     bool

	// Track recent output to deduplicate spinner/animation frames
	lastOutput string
	dupCount   int
}

// ANSI escape code patterns for stripping
var transcriptAnsiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[[0-9;]*m|\x1b\[\?[0-9;]*[a-zA-Z]`)

// Patterns that indicate animation/spinner frames to deduplicate
var animationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^[\s⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷|/\-\\]+$`), // spinner characters
	regexp.MustCompile(`^\s*\d+%`), // progress percentages
}

// NewTranscriptRecorder creates a recorder that calls flushFn to persist data.
func NewTranscriptRecorder(flushFn TranscriptFlushFunc) *TranscriptRecorder {
	tr := &TranscriptRecorder{
		flushFn: flushFn,
	}
	tr.startFlushTimer()
	return tr
}

func (tr *TranscriptRecorder) startFlushTimer() {
	tr.flushTimer = time.AfterFunc(TranscriptFlushInterval, func() {
		tr.mu.Lock()
		defer tr.mu.Unlock()
		if tr.stopped {
			return
		}
		tr.flush()
		tr.startFlushTimer()
	})
}

// RecordOutput adds terminal output to the transcript.
func (tr *TranscriptRecorder) RecordOutput(data []byte) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.stopped {
		return
	}

	cleaned := cleanForTranscript(string(data))
	if cleaned == "" {
		return
	}

	// Deduplicate animation frames
	if isAnimationFrame(cleaned) {
		if cleaned == tr.lastOutput {
			tr.dupCount++
			return
		}
	}

	// If we had duplicates, log a summary
	if tr.dupCount > 0 {
		tr.buffer = append(tr.buffer, TranscriptEntry{
			Timestamp: time.Now().UnixMilli(),
			Type:      "output",
			Text:      "[repeated " + strings.Repeat(".", tr.dupCount) + "]",
		})
		tr.dupCount = 0
	}

	tr.lastOutput = cleaned
	entry := TranscriptEntry{
		Timestamp: time.Now().UnixMilli(),
		Type:      "output",
		Text:      cleaned,
	}
	tr.buffer = append(tr.buffer, entry)
	tr.bufferBytes += len(cleaned)

	if tr.bufferBytes >= TranscriptFlushThreshold {
		tr.flush()
	}
}

// RecordInput adds user input to the transcript.
func (tr *TranscriptRecorder) RecordInput(data []byte) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.stopped {
		return
	}

	text := string(data)
	// Skip control characters (arrow keys, etc.) — only record printable input
	if len(text) == 1 && text[0] < 32 && text[0] != '\n' && text[0] != '\r' {
		return
	}

	entry := TranscriptEntry{
		Timestamp: time.Now().UnixMilli(),
		Type:      "input",
		Text:      text,
	}
	tr.buffer = append(tr.buffer, entry)
	tr.bufferBytes += len(text)

	if tr.bufferBytes >= TranscriptFlushThreshold {
		tr.flush()
	}
}

// flush writes buffered entries as JSONL to the flush function.
// Must be called with mu held.
func (tr *TranscriptRecorder) flush() {
	if len(tr.buffer) == 0 {
		return
	}

	var lines []byte
	for _, entry := range tr.buffer {
		jsonLine, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		lines = append(lines, jsonLine...)
		lines = append(lines, '\n')
	}

	tr.buffer = tr.buffer[:0]
	tr.bufferBytes = 0

	if len(lines) > 0 && tr.flushFn != nil {
		go tr.flushFn(lines)
	}
}

// Stop flushes remaining data and stops the recorder.
func (tr *TranscriptRecorder) Stop() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.stopped = true
	if tr.flushTimer != nil {
		tr.flushTimer.Stop()
	}
	tr.flush()
}

// cleanForTranscript strips ANSI codes and normalizes whitespace.
func cleanForTranscript(text string) string {
	// Strip ANSI escape codes
	cleaned := transcriptAnsiRegex.ReplaceAllString(text, "")
	// Remove carriage returns (keep newlines)
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	// Collapse multiple blank lines
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(cleaned)
}

// isAnimationFrame checks if the text looks like a spinner or progress update.
func isAnimationFrame(text string) bool {
	for _, pat := range animationPatterns {
		if pat.MatchString(text) {
			return true
		}
	}
	return false
}
