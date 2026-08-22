package service

import "testing"

func TestAdaptiveCNProtocolBaseURLs(t *testing.T) {
	account := &Account{
		Platform: PlatformDeepseek,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_protocol": APIProtocolAdaptive,
			"api_base_urls": map[string]any{
				APIProtocolChatCompletions: "https://chat.example/v1",
				APIProtocolAnthropic:       "https://anthropic.example",
				APIProtocolResponses:       "https://responses.example",
			},
		},
	}
	if !account.IsAdaptiveAPIProtocol() {
		t.Fatal("adaptive protocol was not recognized")
	}
	if got := account.GetOpenAIBaseURL(); got != "https://chat.example/v1" {
		t.Fatalf("chat base URL = %q", got)
	}
	if got := account.GetCNProtocolBaseURL(APIProtocolAnthropic); got != "https://anthropic.example" {
		t.Fatalf("anthropic base URL = %q", got)
	}
	if got := account.GetCNProtocolBaseURL(APIProtocolResponses); got != "https://responses.example" {
		t.Fatalf("responses base URL = %q", got)
	}
}

func TestAdaptiveCNProtocolDefaults(t *testing.T) {
	account := &Account{Platform: PlatformKimi, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_protocol": APIProtocolAdaptive,
		"account_mode": AccountModeCoding,
	}}
	if got := account.GetCNProtocolBaseURL(APIProtocolChatCompletions); got != DefaultKimiCodingBaseURL {
		t.Fatalf("chat default = %q", got)
	}
	if got := account.GetCNProtocolBaseURL(APIProtocolAnthropic); got != DefaultKimiCodingAnthropicBaseURL {
		t.Fatalf("anthropic default = %q", got)
	}
}
