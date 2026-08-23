package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const (
	openAITeamLinkedErrorDedupTTL      = 60 * time.Second
	openAITeamLinkedErrorFanoutTimeout = 30 * time.Second
	openAITeamLinkedErrorBlockReason   = "team_linked_error"
)

// maybeHandleOpenAITeamLinkedError 在 Team 工作区停用时联动阻断同工作区账号。
func (s *RateLimitService) maybeHandleOpenAITeamLinkedError(ctx context.Context, account *Account, statusCode int, responseBody []byte) {
	if s == nil || s.accountRepo == nil || statusCode != http.StatusPaymentRequired || !isOpenAIOAuthAccount(account) {
		return
	}
	if gjson.GetBytes(responseBody, "detail.code").String() != "deactivated_workspace" {
		return
	}
	teamID := strings.TrimSpace(account.GetChatGPTAccountID())
	if teamID == "" || !s.markOpenAITeamLinkedFired(teamID) {
		return
	}
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAITeamLinkedErrorFanoutTimeout)
	defer cancel()
	accounts, err := s.accountRepo.ListByPlatform(opCtx, PlatformOpenAI)
	if err != nil {
		slog.Warn("openai_team_linked_error_list_failed", "trigger_account_id", account.ID, "error", err)
		return
	}
	targets := make([]*Account, 0)
	for i := range accounts {
		candidate := &accounts[i]
		if candidate.ID == account.ID || candidate.IsShadow() || strings.TrimSpace(candidate.GetChatGPTAccountID()) != teamID {
			continue
		}
		targets = append(targets, candidate)
	}
	for _, candidate := range targets {
		s.notifyAccountSchedulingBlocked(candidate, time.Time{}, openAITeamLinkedErrorBlockReason)
	}
	errorMessage := fmt.Sprintf("Workspace deactivated (402): team-linked error triggered by account #%d", account.ID)
	for _, candidate := range targets {
		if err := s.accountRepo.SetError(opCtx, candidate.ID, errorMessage); err != nil {
			slog.Warn("openai_team_linked_error_set_error_failed", "account_id", candidate.ID, "error", err)
		}
	}
}

// markOpenAITeamLinkedFired 保证同一 Team 在去重窗口内只执行一次联动扫描。
func (s *RateLimitService) markOpenAITeamLinkedFired(teamID string) bool {
	now := time.Now()
	s.openaiTeamLinkedMu.Lock()
	defer s.openaiTeamLinkedMu.Unlock()
	if expiry, ok := s.openaiTeamLinkedRecent[teamID]; ok && expiry.After(now) {
		return false
	}
	if s.openaiTeamLinkedRecent == nil {
		s.openaiTeamLinkedRecent = make(map[string]time.Time)
	}
	for key, expiry := range s.openaiTeamLinkedRecent {
		if !expiry.After(now) {
			delete(s.openaiTeamLinkedRecent, key)
		}
	}
	s.openaiTeamLinkedRecent[teamID] = now.Add(openAITeamLinkedErrorDedupTTL)
	return true
}
