package service

// Claude 原生额度手动重置移植自 korkin25 的上游 PR #7591；保留幂等、账号/组织租约及持久阻断记录，由服务端选择机会。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	claudeResetOperationScope = "claude_reset_redeem"
	claudeResetFenceScope     = "claude_reset_org_fence"
	claudeResetProfileURL     = "https://api.anthropic.com/api/oauth/profile"
	claudeResetRedeemURLFmt   = "https://api.anthropic.com/api/organizations/%s/reset_rate_limits"
	claudeResetLeaseTTL       = 90 * time.Second
	claudeResetRecordTTL      = 365 * 24 * time.Hour
	// 未确认的结果暂时阻止同组织再次操作；阻断结束后重新查询上游资格。
	claudeResetUnknownFenceTTL = 24 * time.Hour
	// 明确且完整的 unavailable 表示未成功领取，仅短暂阻断，避免组织无谓锁定一天。
	claudeResetUnavailableFenceTTL = 15 * time.Minute
	claudeResetReasonUnavailable   = "upstream_unavailable"

	ClaudeResetOutcomeReset       = "reset"
	ClaudeResetOutcomeAlreadyUsed = "already_used"
	ClaudeResetOutcomeNotLimited  = "not_limited"
	ClaudeResetOutcomeCooldown    = "cooldown"
	ClaudeResetOutcomeIneligible  = "ineligible"
	ClaudeResetOutcomeUnknown     = "unknown"
)

// 客户端仅接收已知原因和窗口名称，禁止通过上游自由值回显 grant 等标识。
var (
	claudeResetKnownReasons = map[string]bool{
		"no_grant": true, "unknown_grant": true, "not_next_grant": true, "grant_id_required": true,
		"tenure": true, "other_experiment": true, "stamp_indeterminate": true, "reset_unconfirmed": true,
		"authorization_rejected": true, "claim_unconfirmed": true, claudeResetReasonUnavailable: true,
		"result_persistence_failed": true,
	}
	claudeResetKnownWindows = map[string]bool{"five_hour": true, "seven_day": true, "seven_day_overage_included": true}
)

// ClaudeResetOutcome 不包含 grant、组织或上游请求 ID。
type ClaudeResetOutcome struct {
	Outcome       string              `json:"outcome"`
	Reason        string              `json:"reason,omitempty"`
	Cleared       []string            `json:"cleared,omitempty"`
	CooldownUntil *time.Time          `json:"cooldown_until,omitempty"`
	Credits       *ClaudeResetCredits `json:"credits,omitempty"`
	Replayed      bool                `json:"replayed"`
}

// claudeResetFence 按组织持久化，重复本地账号共用，账号编辑也不能清除。
type claudeResetFence struct {
	Operation string    `json:"operation"`
	Outcome   string    `json:"outcome"`
	Reason    string    `json:"reason,omitempty"`
	At        time.Time `json:"at"`
}

// ConfigureRedemption 同时装配两个必要存储，缺少任何一个都拒绝消费。
func (s *ClaudeResetCreditService) ConfigureRedemption(idem *IdempotencyCoordinator, locks LeaderLockCache) {
	s.idempotency = idem
	s.locks = locks
}

// Redeem 仅在新查询确认资格时消费下一机会；同一确认 key 只重放结果，不重复提交。
// @project-doc docs/operations/account_maintenance.md#claude_native_quota_reset
func (s *ClaudeResetCreditService) Redeem(ctx context.Context, id int64, key string) (*ClaudeResetOutcome, error) {
	if strings.TrimSpace(key) == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	normalized, err := NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	if s.idempotency == nil || s.idempotency.repo == nil || s.locks == nil {
		return nil, ErrIdempotencyStoreUnavail
	}
	// 进入幂等流程前重验账号，已删除或类型改变不能重放，且不持久化 token。
	if _, _, _, err = s.account(ctx, id); err != nil {
		return nil, err
	}
	operation := HashIdempotencyKey(fmt.Sprintf("claude-reset:%d:%s", id, normalized))
	result, err := s.idempotency.Execute(ctx, IdempotencyExecuteOptions{
		Scope: claudeResetOperationScope, ActorScope: fmt.Sprintf("account:%d", id), Method: http.MethodPost,
		Route: "/admin/accounts/:id/claude/reset-credits/redeem", IdempotencyKey: operation,
		Payload: map[string]any{"account_id": id}, TTL: claudeResetRecordTTL, RequireKey: true, ExecutionTimeout: 60 * time.Second,
	}, func(exec context.Context) (any, error) { return s.redeemOnce(exec, id, operation) })
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}
	var outcome ClaudeResetOutcome
	if err = json.Unmarshal(raw, &outcome); err != nil {
		return nil, err
	}
	outcome.Replayed = outcome.Replayed || result.Replayed
	return &outcome, nil
}

func (s *ClaudeResetCreditService) lease(ctx context.Context, key, owner string) (func(), error) {
	acquired, err := s.locks.TryAcquireLeaderLock(ctx, key, owner, claudeResetLeaseTTL)
	if err != nil {
		return nil, infraerrors.ServiceUnavailable("CLAUDE_RESET_LOCK_UNAVAILABLE", "reset coordination unavailable")
	}
	if !acquired {
		return nil, infraerrors.Conflict("CLAUDE_RESET_BUSY", "another reset is in progress")
	}
	return func() {
		release, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.locks.ReleaseLeaderLock(release, key, owner)
	}, nil
}

func (s *ClaudeResetCreditService) redeemOnce(ctx context.Context, id int64, operation string) (*ClaudeResetOutcome, error) {
	owner := uuid.NewString()
	release, err := s.lease(ctx, fmt.Sprintf("claude:reset-credit:account:%d", id), owner)
	if err != nil {
		return nil, err
	}
	defer release()
	account, token, proxy, err := s.account(ctx, id)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, claudeResetHTTPAccountKey{}, account)
	org, err := s.organization(ctx, token, proxy)
	if err != nil {
		return nil, err
	}
	orgHash := HashIdempotencyKey("claude-org:" + org)
	releaseOrg, err := s.lease(ctx, "claude:reset-credit:organization:"+orgHash, owner)
	if err != nil {
		return nil, err
	}
	defer releaseOrg()

	fence, err := s.loadFence(ctx, orgHash)
	if err != nil {
		return nil, err
	}
	var prior claudeResetFence
	if fence.ResponseBody != nil {
		if json.Unmarshal([]byte(*fence.ResponseBody), &prior) != nil || prior.Operation == "" {
			return nil, infraerrors.Conflict("CLAUDE_RESET_UNRESOLVED", "previous reset requires reconciliation")
		}
		if prior.Operation == operation {
			// 同一确认曾崩溃或被中断，保持重放且绝不重发。
			return &ClaudeResetOutcome{Outcome: prior.Outcome, Reason: prior.Reason, Replayed: true}, nil
		}
		if prior.Outcome == ClaudeResetOutcomeUnknown && prior.Reason == claudeResetReasonUnavailable {
			if s.now().Before(prior.At.Add(claudeResetUnavailableFenceTTL)) {
				return nil, infraerrors.Conflict("CLAUDE_RESET_UPSTREAM_UNAVAILABLE", "reset service was unavailable; retry after a while")
			}
		} else if prior.Outcome == ClaudeResetOutcomeUnknown && s.now().Before(prior.At.Add(claudeResetUnknownFenceTTL)) {
			return nil, infraerrors.Conflict("CLAUDE_RESET_UNRESOLVED", "previous reset outcome is unconfirmed; redemption is blocked for now")
		}
	}

	// 不可逆调用前再次查询资格，仅由服务端选择上游指定的下一机会。
	block, err := s.fetchBlock(ctx, token, proxy)
	if err != nil {
		return nil, err
	}
	var grant *claudeResetGrant
	if block != nil {
		for i := range block.Grants {
			if block.Grants[i].ID == block.NextGrantID && claudeResetGrantRedeemable(block, block.Grants[i], s.now()) {
				grant = &block.Grants[i]
				break
			}
		}
	}
	if grant == nil {
		return nil, infraerrors.Conflict("CLAUDE_RESET_NOT_AVAILABLE", "no reset is redeemable right now")
	}

	// 发送前先持久化未知标记，崩溃后阻止重发及再次领取，直到状态明确或阻断到期。
	marker := claudeResetFence{Operation: operation, Outcome: ClaudeResetOutcomeUnknown, Reason: "claim_unconfirmed", At: s.now().UTC()}
	if err = s.persistFence(ctx, fence.ID, marker); err != nil {
		return nil, err
	}
	outcome := s.claim(ctx, token, proxy, org, grant.ID, operation)
	marker.Outcome, marker.Reason = outcome.Outcome, outcome.Reason
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	persistErr := s.persistFence(persistCtx, fence.ID, marker)
	cancel()
	if persistErr != nil {
		return &ClaudeResetOutcome{Outcome: ClaudeResetOutcomeUnknown, Reason: "result_persistence_failed"}, nil
	}
	if outcome.Outcome != ClaudeResetOutcomeUnknown {
		if fresh, e := s.fetchBlock(ctx, token, proxy); e == nil {
			outcome.Credits = projectClaudeResetCredits(fresh, s.now())
		}
	}
	return outcome, nil
}

func (s *ClaudeResetCreditService) loadFence(ctx context.Context, orgHash string) (*IdempotencyRecord, error) {
	repo := s.idempotency.repo
	fence, err := repo.GetByScopeAndKeyHash(ctx, claudeResetFenceScope, orgHash)
	if err != nil {
		return nil, ErrIdempotencyStoreUnavail
	}
	if fence != nil {
		return fence, nil
	}
	row := &IdempotencyRecord{Scope: claudeResetFenceScope, IdempotencyKeyHash: orgHash, RequestFingerprint: orgHash, Status: IdempotencyStatusProcessing, ExpiresAt: s.now().Add(claudeResetRecordTTL)}
	if _, err = repo.CreateProcessing(ctx, row); err != nil {
		return nil, ErrIdempotencyStoreUnavail
	}
	fence, err = repo.GetByScopeAndKeyHash(ctx, claudeResetFenceScope, orgHash)
	if err != nil || fence == nil {
		return nil, ErrIdempotencyStoreUnavail
	}
	return fence, nil
}

func (s *ClaudeResetCreditService) persistFence(ctx context.Context, id int64, marker claudeResetFence) error {
	body, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	if err = s.idempotency.repo.MarkSucceeded(ctx, id, http.StatusOK, string(body), s.now().Add(claudeResetRecordTTL)); err != nil {
		return ErrIdempotencyStoreUnavail
	}
	return nil
}

func (s *ClaudeResetCreditService) organization(ctx context.Context, token, proxy string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, claudeResetProfileURL, nil)
	if err != nil {
		return "", err
	}
	s.headers(ctx, req, token)
	resp, err := s.request(req, proxy)
	if err != nil {
		return "", infraerrors.ServiceUnavailable("CLAUDE_RESET_PROFILE_FAILED", "OAuth profile unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Organization struct {
			UUID string `json:"uuid"`
		} `json:"organization"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body) != nil {
		return "", infraerrors.New(http.StatusBadGateway, "CLAUDE_RESET_PROFILE_FAILED", "OAuth profile unavailable")
	}
	parsed, err := uuid.Parse(body.Organization.UUID)
	if err != nil || parsed == uuid.Nil {
		return "", infraerrors.New(http.StatusBadGateway, "CLAUDE_RESET_ORGANIZATION_INVALID", "OAuth organization unavailable")
	}
	return parsed.String(), nil
}

// claim 只发送一次不可逆请求；非完整已知结果一律记为未知，阻止盲目重试。
func (s *ClaudeResetCreditService) claim(ctx context.Context, token, proxy, org, grantID, operation string) *ClaudeResetOutcome {
	unknown := &ClaudeResetOutcome{Outcome: ClaudeResetOutcomeUnknown, Reason: "claim_unconfirmed"}
	// 每次确认生成确定的 64 位十六进制标识，符合上游允许字符和长度。
	body, err := json.Marshal(map[string]string{"program": "cedar_ember", "grant_id": grantID, "request_id": operation})
	if err != nil {
		return unknown
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(claudeResetRedeemURLFmt, org), bytes.NewReader(body))
	if err != nil {
		return unknown
	}
	s.headers(ctx, req, token)
	resp, err := s.request(req, proxy)
	if err != nil {
		return unknown
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &ClaudeResetOutcome{Outcome: ClaudeResetOutcomeIneligible, Reason: "authorization_rejected"}
	}
	if resp.StatusCode != http.StatusOK {
		return unknown
	}
	var result struct {
		Result        string     `json:"result"`
		Reason        string     `json:"reason"`
		Cleared       []string   `json:"cleared"`
		CooldownUntil *time.Time `json:"cooldown_until"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result) != nil {
		return unknown
	}
	reason := ""
	if claudeResetKnownReasons[result.Reason] {
		reason = result.Reason
	}
	if reason == "stamp_indeterminate" || reason == "reset_unconfirmed" {
		return unknown
	}
	switch result.Result {
	case ClaudeResetOutcomeReset, ClaudeResetOutcomeAlreadyUsed, ClaudeResetOutcomeNotLimited, ClaudeResetOutcomeCooldown, ClaudeResetOutcomeIneligible:
		out := &ClaudeResetOutcome{Outcome: result.Result, Reason: reason, CooldownUntil: result.CooldownUntil}
		for _, w := range result.Cleared {
			if claudeResetKnownWindows[w] {
				out.Cleared = append(out.Cleared, w)
			}
		}
		return out
	case "unavailable":
		return &ClaudeResetOutcome{Outcome: ClaudeResetOutcomeUnknown, Reason: claudeResetReasonUnavailable}
	default:
		return unknown
	}
}
