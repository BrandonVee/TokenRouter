package service

import (
	"context"
	"errors"
	"log"

	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
	"github.com/BrandonVee/TokenRouter/internal/util/logredact"
)

const MaxBulkSubscriptionActions = 100

// BulkSubscriptionActionInput 对数量有界的订阅集合应用同一操作。
type BulkSubscriptionActionInput struct {
	SubscriptionIDs []int64 `json:"subscription_ids"`
	Action          string  `json:"action"`
	Days            int     `json:"days,omitempty"`
	Daily           bool    `json:"daily,omitempty"`
	Weekly          bool    `json:"weekly,omitempty"`
	Monthly         bool    `json:"monthly,omitempty"`
}

// Validate 在修改任何订阅之前完整校验请求。
func (input *BulkSubscriptionActionInput) Validate() error {
	if input == nil {
		return ErrSubscriptionNilInput
	}
	if len(input.SubscriptionIDs) == 0 || len(input.SubscriptionIDs) > MaxBulkSubscriptionActions {
		return infraerrors.BadRequest("INVALID_BULK_SUBSCRIPTIONS", "subscription_ids must contain between 1 and 100 IDs")
	}
	for _, id := range input.SubscriptionIDs {
		if id <= 0 {
			return infraerrors.BadRequest("INVALID_SUBSCRIPTION_ID", "subscription IDs must be positive")
		}
	}
	switch input.Action {
	case "extend":
		if input.Days == 0 || input.Days < -MaxValidityDays || input.Days > MaxValidityDays {
			return infraerrors.BadRequest("INVALID_ADJUSTMENT_DAYS", "days must be nonzero and between -36500 and 36500")
		}
	case "reset_quota":
		if !input.Daily && !input.Weekly && !input.Monthly {
			return ErrInvalidInput
		}
	case "revoke", "restore":
	default:
		return infraerrors.BadRequest("INVALID_SUBSCRIPTION_ACTION", "action must be extend, reset_quota, revoke, or restore")
	}
	return nil
}

type BulkSubscriptionActionItemResult struct {
	SubscriptionID int64  `json:"subscription_id"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

type BulkSubscriptionActionResult struct {
	SuccessCount int                                `json:"success_count"`
	FailedCount  int                                `json:"failed_count"`
	Results      []BulkSubscriptionActionItemResult `json:"results"`
}

// BulkSubscriptionAction 保持输入顺序并去重，逐项失败保留在结果中，方便调用方保留已完成部分。
func (s *SubscriptionService) BulkSubscriptionAction(ctx context.Context, input *BulkSubscriptionActionInput) (*BulkSubscriptionActionResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	result := &BulkSubscriptionActionResult{
		Results: make([]BulkSubscriptionActionItemResult, 0, len(input.SubscriptionIDs)),
	}
	seen := make(map[int64]struct{}, len(input.SubscriptionIDs))
	for _, id := range input.SubscriptionIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		err := ctx.Err()
		if err == nil {
			// 单项服务可能在首次写入后再次读取或改状态；将这些步骤放在同一事务内，避免失败重试重复延期或重置。
			err = s.withSubscriptionMutationTx(ctx, func(txCtx context.Context) error {
				var mutationErr error
				switch input.Action {
				case "extend":
					_, mutationErr = s.ExtendSubscription(txCtx, id, input.Days)
				case "reset_quota":
					_, mutationErr = s.AdminResetQuota(txCtx, id, input.Daily, input.Weekly, input.Monthly)
				case "revoke":
					_, mutationErr = s.userSubRepo.GetByID(txCtx, id)
					if mutationErr == nil {
						mutationErr = s.RevokeSubscription(txCtx, id)
					}
				case "restore":
					_, mutationErr = s.RestoreSubscription(txCtx, id)
				}
				return mutationErr
			})

		}
		item := BulkSubscriptionActionItemResult{SubscriptionID: id, Success: err == nil}
		if err != nil {
			result.FailedCount++
			item.Error = infraerrors.Message(err)
			if errors.Is(err, context.Canceled) {
				item.Error = context.Canceled.Error()
			} else if errors.Is(err, context.DeadlineExceeded) {
				item.Error = context.DeadlineExceeded.Error()
			} else if infraerrors.Code(err) >= 500 {
				log.Printf("[SubscriptionBulkAction] action=%s subscription_id=%d error=%s", input.Action, id, logredact.RedactText(err.Error()))
			}
		} else {
			result.SuccessCount++
		}
		result.Results = append(result.Results, item)
	}
	return result, nil
}
