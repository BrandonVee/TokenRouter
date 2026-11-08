package service

import "context"

// OpenAIReferralCall 向传输适配器交接已刷新的账号认证和代理；请求头不得记录或持久化。
type OpenAIReferralCall struct {
	accountContext *openAIQuotaAccountContext
	ProxyURL       string
	Headers        map[string]string
	ProgramID      string
}

// OpenAIReferralClient 分离邀请策略和传输；查询成功返回非空结果，发送不重试且仅在确认单次邀请成功后返回 nil。
type OpenAIReferralClient interface {
	QueryEligibility(context.Context, OpenAIReferralCall) (*OpenAIReferralEligibility, error)
	SendInvite(context.Context, OpenAIReferralCall, string) error
}
