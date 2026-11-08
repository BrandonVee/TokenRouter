// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package antigravity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/BrandonVee/TokenRouter/internal/pkg/servertiming"
)

const quotaSummaryBodyLimit = 1 << 20

type QuotaSummaryBucket struct {
	BucketID          string  `json:"bucketId"`
	DisplayName       string  `json:"displayName,omitempty"`
	Window            string  `json:"window"`
	ResetTime         string  `json:"resetTime,omitempty"`
	RemainingFraction float64 `json:"remainingFraction"`
}

type QuotaSummaryGroup struct {
	DisplayName string               `json:"displayName"`
	Description string               `json:"description,omitempty"`
	Buckets     []QuotaSummaryBucket `json:"buckets"`
}

type QuotaSummaryResponse struct {
	Groups      []QuotaSummaryGroup `json:"groups"`
	Description string              `json:"description,omitempty"`
}

func (c *Client) RetrieveUserQuotaSummary(ctx context.Context, accessToken, projectID string) (*QuotaSummaryResponse, error) {
	var lastErr error
	var summaries []*QuotaSummaryResponse
	for _, baseURL := range BaseURLs {
		if ctx.Err() != nil {
			if len(summaries) == 0 {
				return nil, ctx.Err()
			}
			break
		}
		summary, _, err := c.retrieveUserQuotaSummary(ctx, baseURL, accessToken, projectID)
		if err == nil {
			DefaultURLAvailability.MarkSuccess(baseURL)
			summaries = append(summaries, summary)
			continue
		}
		lastErr = err
	}
	if len(summaries) > 0 {
		return mergeQuotaSummaries(summaries), nil
	}
	return nil, lastErr
}

// mergeQuotaSummaries 合并 prod/daily 的真实窗口，同一 bucket 使用最小剩余额度。
// 不同端点的 Gemini 用量可能分别记账，不能因 prod 先返回 200 就永远显示近乎未使用。
func mergeQuotaSummaries(summaries []*QuotaSummaryResponse) *QuotaSummaryResponse {
	merged := &QuotaSummaryResponse{}
	type location struct{ group, bucket int }
	locations := make(map[string]location)
	groupByName := make(map[string]int)
	for _, summary := range summaries {
		if summary == nil {
			continue
		}
		if merged.Description == "" {
			merged.Description = summary.Description
		}
		for _, group := range summary.Groups {
			for _, bucket := range group.Buckets {
				if bucket.BucketID == "" {
					continue
				}
				key := bucket.BucketID + "\x00" + bucket.Window
				if at, exists := locations[key]; exists {
					current := &merged.Groups[at.group].Buckets[at.bucket]
					if bucket.RemainingFraction < current.RemainingFraction {
						*current = bucket
					}
					continue
				}
				index, exists := groupByName[group.DisplayName]
				if !exists {
					index = len(merged.Groups)
					groupByName[group.DisplayName] = index
					merged.Groups = append(merged.Groups, QuotaSummaryGroup{DisplayName: group.DisplayName, Description: group.Description})
				}
				locations[key] = location{index, len(merged.Groups[index].Buckets)}
				merged.Groups[index].Buckets = append(merged.Groups[index].Buckets, bucket)
			}
		}
	}
	return merged
}

func (c *Client) RetrieveUserQuotaSummaryFromURL(ctx context.Context, baseURL, accessToken, projectID string) (*QuotaSummaryResponse, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("retrieveUserQuotaSummary base URL is required")
	}
	summary, _, err := c.retrieveUserQuotaSummary(ctx, baseURL, accessToken, projectID)
	return summary, err
}

func (c *Client) retrieveUserQuotaSummary(ctx context.Context, baseURL, accessToken, projectID string) (*QuotaSummaryResponse, int, error) {
	if c == nil || c.httpClient == nil {
		return nil, 0, errors.New("antigravity client is not configured")
	}
	bodyBytes, err := json.Marshal(FetchAvailableModelsRequest{Project: projectID})
	if err != nil {
		return nil, 0, fmt.Errorf("marshal retrieveUserQuotaSummary request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1internal:retrieveUserQuotaSummary", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, 0, fmt.Errorf("create retrieveUserQuotaSummary request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", GetUserAgentForContext(ctx))

	resp, err := servertiming.Do(c.httpClient, req)
	if err != nil {
		return nil, 0, fmt.Errorf("retrieveUserQuotaSummary request failed: %w", err)
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, quotaSummaryBodyLimit+1))
	_ = resp.Body.Close()
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read retrieveUserQuotaSummary response: %w", err)
	}
	if len(respBody) > quotaSummaryBodyLimit {
		return nil, resp.StatusCode, fmt.Errorf("retrieveUserQuotaSummary response exceeds %d bytes", quotaSummaryBodyLimit)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("retrieveUserQuotaSummary failed (HTTP %d): %s", resp.StatusCode, truncateQuotaSummaryBody(respBody))
	}
	var summary QuotaSummaryResponse
	if err := json.Unmarshal(respBody, &summary); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse retrieveUserQuotaSummary response: %w", err)
	}
	return &summary, resp.StatusCode, nil
}

func truncateQuotaSummaryBody(body []byte) string {
	const max = 300
	if len(body) > max {
		return string(body[:max]) + "..."
	}
	return string(body)
}
