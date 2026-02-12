// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdashservice

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

const (
	LearningsFile       = "termdash:learnings"
	LearningsMaxContext = 4000 // max chars to send to Claude for extraction
)

// Learning represents a single extracted insight from a Claude session.
type Learning struct {
	Text      string `json:"text"`
	Source    string `json:"source"`    // block ID where this was extracted from
	Timestamp int64  `json:"timestamp"`
}

// ExtractLearnings analyzes a Claude session's transcript and extracts reusable
// engineering insights using Claude Haiku.
func (s *TermDashService) ExtractLearnings(ctx context.Context, blockId string) ([]string, error) {
	// Read transcript
	_, data, err := filestore.WFS.ReadFile(ctx, blockId, "termdash:transcript")
	if err != nil {
		return nil, fmt.Errorf("error reading transcript: %w", err)
	}

	transcript := string(data)
	if len(transcript) < 100 {
		return nil, fmt.Errorf("transcript too short to extract learnings")
	}

	// Truncate for prompt
	if len(transcript) > LearningsMaxContext {
		transcript = transcript[len(transcript)-LearningsMaxContext:]
	}

	prompt := fmt.Sprintf(
		"Analyze this Claude Code terminal session transcript and extract 3-7 concise, "+
			"reusable engineering insights or patterns. Each insight should be a single sentence "+
			"that would help a developer working on similar code in the future. "+
			"Return ONLY the insights, one per line, no numbering, no bullet points.\n\n%s",
		transcript,
	)

	execCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "claude", "-p", "--model", "haiku")
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("claude command error: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var learnings []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			learnings = append(learnings, line)
		}
	}

	// Store learnings in the block's file store
	if len(learnings) > 0 {
		learningsText := strings.Join(learnings, "\n") + "\n"
		storeCtx, storeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer storeCancel()
		err = filestore.WFS.WriteFile(storeCtx, blockId, LearningsFile, []byte(learningsText))
		if err != nil {
			log.Printf("[termdash:learnings] error storing learnings for block %s: %v\n", blockId, err)
		}
	}

	return learnings, nil
}

// GetLearnings retrieves previously extracted learnings for a block.
func (s *TermDashService) GetLearnings(ctx context.Context, blockId string) ([]string, error) {
	_, data, err := filestore.WFS.ReadFile(ctx, blockId, LearningsFile)
	if err != nil {
		return nil, fmt.Errorf("no learnings found: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var learnings []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			learnings = append(learnings, line)
		}
	}
	return learnings, nil
}

// GetAllLearnings retrieves learnings from all Claude sessions.
func (s *TermDashService) GetAllLearnings(ctx context.Context) ([]string, error) {
	blocks, err := wstore.DBGetAllObjsByType[*waveobj.Block](ctx, waveobj.OType_Block)
	if err != nil {
		return nil, fmt.Errorf("error listing blocks: %w", err)
	}

	var allLearnings []string
	for _, block := range blocks {
		if block.Meta.GetString(waveobj.MetaKey_TermDashType, "") != "claude" {
			continue
		}

		_, data, err := filestore.WFS.ReadFile(ctx, block.OID, LearningsFile)
		if err != nil {
			continue
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				allLearnings = append(allLearnings, line)
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, l := range allLearnings {
		lower := strings.ToLower(l)
		if !seen[lower] {
			seen[lower] = true
			unique = append(unique, l)
		}
	}
	return unique, nil
}

// BuildContextForNewSession builds a system prompt injection from relevant
// learnings for a new Claude session. This is called when creating new
// Claude blocks to inject prior engineering insights.
func (s *TermDashService) BuildContextForNewSession(ctx context.Context, cwd string) (string, error) {
	learnings, err := s.GetAllLearnings(ctx)
	if err != nil || len(learnings) == 0 {
		return "", nil
	}

	// Select most relevant learnings (for now, just take the last N)
	maxLearnings := 10
	if len(learnings) > maxLearnings {
		learnings = learnings[len(learnings)-maxLearnings:]
	}

	var sb strings.Builder
	sb.WriteString("Engineering insights from previous sessions:\n")
	for _, l := range learnings {
		sb.WriteString("- ")
		sb.WriteString(l)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
