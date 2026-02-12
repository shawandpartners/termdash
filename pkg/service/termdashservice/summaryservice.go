// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdashservice

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/panichandler"
	"github.com/wavetermdev/waveterm/pkg/termdash"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

const (
	SummaryPollInterval = 15 * time.Second
	SummaryStartDelay   = 5 * time.Second
	MaxTermOutputBytes  = 4096
	SummaryTimeout      = 10 * time.Second
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b\[[0-9;]*m`)

// StartSummaryLoop starts the background polling loop that generates
// titles for active Claude Code sessions.
func StartSummaryLoop() {
	go func() {
		defer func() {
			panichandler.PanicHandler("termdash:summaryLoop", recover())
		}()
		time.Sleep(SummaryStartDelay)
		for {
			pollClaudeBlocks()
			time.Sleep(SummaryPollInterval)
		}
	}()
}

// pollClaudeBlocks finds all Claude blocks that need a summary generated.
func pollClaudeBlocks() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blocks, err := wstore.DBGetAllObjsByType[*waveobj.Block](ctx, waveobj.OType_Block)
	if err != nil {
		log.Printf("[termdash:summary] error listing blocks: %v\n", err)
		return
	}

	for _, block := range blocks {
		tdType := block.Meta.GetString(waveobj.MetaKey_TermDashType, "")
		if tdType != "claude" {
			continue
		}

		// Skip archived blocks
		if block.Meta.GetBool(waveobj.MetaKey_TermDashArchived, false) {
			continue
		}

		// Skip blocks that already have a summary
		existing := block.Meta.GetString(waveobj.MetaKey_TermDashSummary, "")
		if existing != "" {
			continue
		}

		// Only generate summaries for active/needs-input sessions (not idle/exited)
		status := block.Meta.GetString(waveobj.MetaKey_TermDashStatus, "")
		if status != termdash.StatusActive && status != termdash.StatusNeedsInput {
			continue
		}

		go generateSummary(block.OID)
	}
}

// generateSummary reads terminal output from a block and generates a title.
func generateSummary(blockId string) {
	defer func() {
		panichandler.PanicHandler("termdash:generateSummary", recover())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), SummaryTimeout)
	defer cancel()

	// Read terminal output from block file store
	termOutput, err := readTerminalOutput(ctx, blockId)
	if err != nil {
		log.Printf("[termdash:summary] error reading terminal output for block %s: %v\n", blockId, err)
		return
	}

	if len(termOutput) < 50 {
		// Not enough output to generate a meaningful title
		return
	}

	// Generate title using claude CLI in non-interactive mode
	title, err := generateTitle(ctx, termOutput)
	if err != nil {
		log.Printf("[termdash:summary] error generating title for block %s: %v\n", blockId, err)
		return
	}

	if title == "" {
		return
	}

	// Save title to block metadata
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer updateCancel()
	metaUpdate := waveobj.MetaMapType{
		waveobj.MetaKey_TermDashSummary: title,
	}
	err = wstore.UpdateObjectMeta(updateCtx, waveobj.MakeORef(waveobj.OType_Block, blockId), metaUpdate, false)
	if err != nil {
		log.Printf("[termdash:summary] error saving summary for block %s: %v\n", blockId, err)
		return
	}

	log.Printf("[termdash:summary] generated title for block %s: %q\n", blockId, title)
}

// readTerminalOutput reads the last N bytes from a block's terminal file,
// strips ANSI codes, and returns clean text.
func readTerminalOutput(ctx context.Context, blockId string) (string, error) {
	// Get file stats to know the size
	wfile, err := filestore.WFS.Stat(ctx, blockId, wavebase.BlockFile_Term)
	if err != nil {
		return "", fmt.Errorf("stat error: %w", err)
	}

	readSize := int64(MaxTermOutputBytes)
	offset := int64(0)
	if wfile.Size > readSize {
		offset = wfile.Size - readSize
	} else {
		readSize = wfile.Size
	}

	_, data, err := filestore.WFS.ReadAt(ctx, blockId, wavebase.BlockFile_Term, offset, readSize)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	// Strip ANSI escape codes
	cleaned := ansiRegex.ReplaceAllString(string(data), "")
	// Collapse whitespace
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	return cleaned, nil
}

// generateTitle calls claude CLI in print mode to generate a concise session title.
func generateTitle(ctx context.Context, termOutput string) (string, error) {
	// Truncate for the prompt if needed
	if len(termOutput) > 2000 {
		termOutput = termOutput[:2000]
	}

	prompt := fmt.Sprintf(
		"Generate a concise 3-8 word title for this Claude Code terminal session based on the output below. "+
			"Return ONLY the title, no quotes, no explanation.\n\n%s",
		termOutput,
	)

	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", "haiku")
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude command error: %w", err)
	}

	title := strings.TrimSpace(string(output))

	// Validate: should be short, no newlines
	if strings.Contains(title, "\n") {
		lines := strings.Split(title, "\n")
		title = strings.TrimSpace(lines[0])
	}

	// Cap at reasonable length
	if len(title) > 80 {
		title = title[:80]
	}

	return title, nil
}
