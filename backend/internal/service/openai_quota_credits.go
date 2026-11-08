package service

import (
	"context"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"net/http"
)

const openaiQuotaCreditsKey = "codex_credits_snapshot"

// OpenAICredits 保留 Codex 余额原始十进制精度，与重置次数分离。
type OpenAICredits struct {
	HasCredits bool    `json:"has_credits"`
	Unlimited  bool    `json:"unlimited"`
	Balance    *string `json:"balance"`
}

type openAICreditsSnapshot struct {
	Credits   *OpenAICredits `json:"credits"`
	FetchedAt int64          `json:"fetched_at"`
}

func (s *OpenAIQuotaService) CacheCreditsSnapshot(ctx context.Context, accountID int64, usage *OpenAIQuotaUsage) error {
	if err := scopedAccountIDAccess(ctx, s.accountRepo, accountID, scopedAccountOperation(ctx, "openai.quota.read"), true); err != nil {
		return err
	}
	if usage == nil {
		return infraerrors.New(http.StatusBadGateway, "OPENAI_QUOTA_EMPTY_USAGE", "openai quota query returned an empty result")
	}
	if err := s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{
		openaiQuotaCreditsKey: openAICreditsSnapshot{Credits: usage.Credits, FetchedAt: usage.FetchedAt},
	}); err != nil {
		return infraerrors.New(http.StatusInternalServerError, "OPENAI_QUOTA_CACHE_WRITE_FAILED", "failed to cache Codex credits").WithCause(err)
	}
	return nil
}
