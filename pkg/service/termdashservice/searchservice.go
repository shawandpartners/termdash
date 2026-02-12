// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdashservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/wavetermdev/waveterm/pkg/filestore"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

type TranscriptSearchResult struct {
	BlockId   string `json:"blockid"`
	SessionId string `json:"sessionid"`
	Summary   string `json:"summary"`
	Snippet   string `json:"snippet"` // text around the match
	Offset    int    `json:"offset"`  // character offset of the match
}

// SearchTranscripts searches transcript files across all Claude blocks.
func (s *TermDashService) SearchTranscripts(ctx context.Context, query string) ([]TranscriptSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	blocks, err := wstore.DBGetAllObjsByType[*waveobj.Block](ctx, waveobj.OType_Block)
	if err != nil {
		return nil, fmt.Errorf("error listing blocks: %w", err)
	}

	query = strings.ToLower(query)
	var results []TranscriptSearchResult

	for _, block := range blocks {
		if block.Meta.GetString(waveobj.MetaKey_TermDashType, "") != "claude" {
			continue
		}

		// Read transcript file for this block
		_, data, err := filestore.WFS.ReadFile(ctx, block.OID, "termdash:transcript")
		if err != nil {
			continue // no transcript yet
		}

		content := strings.ToLower(string(data))
		idx := strings.Index(content, query)
		if idx == -1 {
			continue
		}

		// Extract snippet around match
		snippet := extractSnippet(string(data), idx, len(query), 100)

		results = append(results, TranscriptSearchResult{
			BlockId:   block.OID,
			SessionId: block.Meta.GetString(waveobj.MetaKey_TermDashClaudeSession, ""),
			Summary:   block.Meta.GetString(waveobj.MetaKey_TermDashSummary, ""),
			Snippet:   snippet,
			Offset:    idx,
		})
	}
	return results, nil
}

// GetTranscript reads the full transcript for a block, cleaned.
func (s *TermDashService) GetTranscript(ctx context.Context, blockId string) (string, error) {
	_, data, err := filestore.WFS.ReadFile(ctx, blockId, "termdash:transcript")
	if err != nil {
		return "", fmt.Errorf("error reading transcript: %w", err)
	}

	// Parse JSONL and return cleaned text
	lines := strings.Split(string(data), "\n")
	var output strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Simple extraction: each line is JSON with "text" field
		// For performance, do simple string extraction rather than full JSON parse
		textIdx := strings.Index(line, `"text":"`)
		if textIdx == -1 {
			continue
		}
		start := textIdx + 8
		end := strings.LastIndex(line, `"`)
		if end > start {
			text := line[start:end]
			// Unescape basic JSON escapes
			text = strings.ReplaceAll(text, `\"`, `"`)
			text = strings.ReplaceAll(text, `\\`, `\`)
			text = strings.ReplaceAll(text, `\n`, "\n")
			text = strings.ReplaceAll(text, `\t`, "\t")
			output.WriteString(text)
		}
	}
	return output.String(), nil
}

// extractSnippet returns a substring centered around the match position.
func extractSnippet(text string, matchIdx, matchLen, contextLen int) string {
	start := matchIdx - contextLen
	if start < 0 {
		start = 0
	}
	end := matchIdx + matchLen + contextLen
	if end > len(text) {
		end = len(text)
	}
	snippet := text[start:end]
	// Clean up for display
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.Join(strings.Fields(snippet), " ")
	return snippet
}
