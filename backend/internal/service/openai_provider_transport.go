package service

import "net/http"

// doOpenAIUpstream 统一多协议供应商的出站传输，复用账号代理、并发隔离与 fork 的 TLS 路由。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	scheduleOpenCodeGoUsageActivity(s.deferredService, account)
	return s.httpUpstream.DoWithTLS(request, proxyURL, account.ID, account.Concurrency, s.resolveOpenAITLSProfile(account))
}
