// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package termdashservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wstore"
)

type TermDashService struct{}

type ArchivedSession struct {
	BlockId    string `json:"blockid"`
	SessionId  string `json:"sessionid"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	ArchivedAt int64  `json:"archivedat"`
}

// ArchiveBlock marks a Claude block as archived with a timestamp.
func (s *TermDashService) ArchiveBlock(ctx context.Context, blockId string) error {
	metaUpdate := waveobj.MetaMapType{
		waveobj.MetaKey_TermDashArchived:   true,
		waveobj.MetaKey_TermDashArchivedAt: time.Now().UnixMilli(),
	}
	return wstore.UpdateObjectMeta(ctx, waveobj.MakeORef(waveobj.OType_Block, blockId), metaUpdate, false)
}

// UnarchiveBlock removes the archived flag from a block.
func (s *TermDashService) UnarchiveBlock(ctx context.Context, blockId string) error {
	metaUpdate := waveobj.MetaMapType{
		waveobj.MetaKey_TermDashArchived:   nil,
		waveobj.MetaKey_TermDashArchivedAt: nil,
	}
	return wstore.UpdateObjectMeta(ctx, waveobj.MakeORef(waveobj.OType_Block, blockId), metaUpdate, false)
}

// ListArchivedSessions returns all archived Claude sessions.
func (s *TermDashService) ListArchivedSessions(ctx context.Context) ([]ArchivedSession, error) {
	blocks, err := wstore.DBGetAllObjsByType[*waveobj.Block](ctx, waveobj.OType_Block)
	if err != nil {
		return nil, fmt.Errorf("error listing blocks: %w", err)
	}

	var archived []ArchivedSession
	for _, block := range blocks {
		if block.Meta.GetString(waveobj.MetaKey_TermDashType, "") != "claude" {
			continue
		}
		if !block.Meta.GetBool(waveobj.MetaKey_TermDashArchived, false) {
			continue
		}
		archived = append(archived, ArchivedSession{
			BlockId:    block.OID,
			SessionId:  block.Meta.GetString(waveobj.MetaKey_TermDashClaudeSession, ""),
			Summary:    block.Meta.GetString(waveobj.MetaKey_TermDashSummary, ""),
			Status:     block.Meta.GetString(waveobj.MetaKey_TermDashStatus, ""),
			ArchivedAt: int64(block.Meta.GetFloat(waveobj.MetaKey_TermDashArchivedAt, 0)),
		})
	}
	return archived, nil
}

// ListActiveSessions returns all active (non-archived) Claude sessions.
func (s *TermDashService) ListActiveSessions(ctx context.Context) ([]ArchivedSession, error) {
	blocks, err := wstore.DBGetAllObjsByType[*waveobj.Block](ctx, waveobj.OType_Block)
	if err != nil {
		return nil, fmt.Errorf("error listing blocks: %w", err)
	}

	var active []ArchivedSession
	for _, block := range blocks {
		if block.Meta.GetString(waveobj.MetaKey_TermDashType, "") != "claude" {
			continue
		}
		if block.Meta.GetBool(waveobj.MetaKey_TermDashArchived, false) {
			continue
		}
		active = append(active, ArchivedSession{
			BlockId:   block.OID,
			SessionId: block.Meta.GetString(waveobj.MetaKey_TermDashClaudeSession, ""),
			Summary:   block.Meta.GetString(waveobj.MetaKey_TermDashSummary, ""),
			Status:    block.Meta.GetString(waveobj.MetaKey_TermDashStatus, ""),
		})
	}
	return active, nil
}

// SearchSessions searches archived and active sessions by summary text.
func (s *TermDashService) SearchSessions(ctx context.Context, query string) ([]ArchivedSession, error) {
	blocks, err := wstore.DBGetAllObjsByType[*waveobj.Block](ctx, waveobj.OType_Block)
	if err != nil {
		return nil, fmt.Errorf("error listing blocks: %w", err)
	}

	query = strings.ToLower(query)
	var results []ArchivedSession
	for _, block := range blocks {
		if block.Meta.GetString(waveobj.MetaKey_TermDashType, "") != "claude" {
			continue
		}
		summary := block.Meta.GetString(waveobj.MetaKey_TermDashSummary, "")
		if summary != "" && strings.Contains(strings.ToLower(summary), query) {
			results = append(results, ArchivedSession{
				BlockId:    block.OID,
				SessionId:  block.Meta.GetString(waveobj.MetaKey_TermDashClaudeSession, ""),
				Summary:    summary,
				Status:     block.Meta.GetString(waveobj.MetaKey_TermDashStatus, ""),
				ArchivedAt: int64(block.Meta.GetFloat(waveobj.MetaKey_TermDashArchivedAt, 0)),
			})
		}
	}
	return results, nil
}
