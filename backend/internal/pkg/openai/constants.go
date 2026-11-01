// Package openai 提供 OpenAI 集成的模型常量与辅助函数。
package openai

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Model 表示 OpenAI 模型。
type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// DefaultModels 是默认 OpenAI 模型目录。
var DefaultModels = []Model{
	{ID: "gpt-5.6-sol", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Sol"},
	{ID: "gpt-6", Object: "model", Created: 1788480000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 (Astra)"},
	{ID: "gpt-5.6", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 (Sol)"},
	{ID: "gpt-5.6-terra", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Terra"},
	{ID: "gpt-5.6-luna", Object: "model", Created: 1780876800, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.6 Luna"},
	{ID: "gpt-6.1-terra", Object: "model", Created: 1704067200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6.1 Terra"},
	{ID: "gpt-6.1-sol", Object: "model", Created: 1790640000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6.1 Sol"},
	{ID: "gpt-6-sol", Object: "model", Created: 1790035200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 Sol"},
	{ID: "gpt-6-luna", Object: "model", Created: 1790035200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 Luna"},
	{ID: "gpt-6-astra", Object: "model", Created: 1788480000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-6 Astra"},
	{ID: "gpt-5.5", Object: "model", Created: 1776873600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.5"},
	{ID: "gpt-5.4", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4"},
	{ID: "gpt-5.4-mini", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.4 Mini"},
	{ID: "gpt-5.3-codex-spark", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.3 Codex Spark"},
	{ID: "codex-auto-review", Object: "model", Created: 1776902400, OwnedBy: "openai", Type: "model", DisplayName: "Codex Auto Review"},
	{ID: "gpt-5.2", Object: "model", Created: 1733875200, OwnedBy: "openai", Type: "model", DisplayName: "GPT-5.2"},
	{ID: "gpt-image-1", Object: "model", Created: 1733875200, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 1"},
	{ID: "gpt-image-1.5", Object: "model", Created: 1735689600, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 1.5"},
	{ID: "gpt-image-2", Object: "model", Created: 1738368000, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 2"},
	{ID: "gpt-image-2.5-flare", Object: "model", Created: 1788825600, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 2.5 Flare"},
	{ID: "gpt-image-2.5-sunburst", Object: "model", Created: 1788825600, OwnedBy: "openai", Type: "model", DisplayName: "GPT Image 2.5 Sunburst"},
}

// DefaultModelIDs 返回默认模型 ID 列表。
func DefaultModelIDs() []string {
	ids := make([]string, len(DefaultModels))
	for i, m := range DefaultModels {
		ids[i] = m.ID
	}
	return ids
}

// DefaultTestModel 是账号连接测试默认模型。
const DefaultTestModel = "gpt-5.4"

// CodexUsageProbeModel 是 OAuth Codex 用量探测模型。
const CodexUsageProbeModel = "codex-auto-review"

// DefaultInstructions 为 Codex 请求提供基础提示词，是否注入由调用方裁定。
//
//go:embed instructions.txt
var DefaultInstructions string

// 对应各代模型的官方 Codex 基础提示词；缺少专属文本时沿用回退链。
//
//go:embed instructions_gpt5_1.txt
var instructionsGPT51 string

//go:embed instructions_gpt5_2.txt
var instructionsGPT52 string

//go:embed instructions_gpt5_5.txt
var instructionsGPT55 string

// 来源：openai/codex 的 codex-rs/models-manager/models.json，提交 121f91fd5d9d。
//
//go:embed instructions_gpt6_astra.txt
var instructionsGPT6Astra string

// CodexGPT61SolMetadata 保存官方 GPT-6.1 Sol 完整描述，来源为 openai/codex 提交 b1e72963c3b71a9265a551e54beff078384efed9。
//
//go:embed codex_gpt61_sol.json
var CodexGPT61SolMetadata []byte

// latestCodexInstructions 返回当前已知最新版本的 Codex base instructions，
// 当前为 GPT-5.5；若 5.5 prompt 意外为空则回退到 DefaultInstructions 保证非空。
func latestCodexInstructions() string {
	if v := strings.TrimSpace(instructionsGPT55); v != "" {
		return instructionsGPT55
	}
	return DefaultInstructions
}

// CanonicalizeOpenAIModelAliasSpelling 统一提供商前缀、大小写、分隔符和已知紧凑别名。
func CanonicalizeOpenAIModelAliasSpelling(model string) string {
	model = strings.TrimSpace(model)
	if slash := strings.LastIndexByte(model, '/'); slash >= 0 {
		model = strings.TrimSpace(model[slash+1:])
	}
	model = strings.ToLower(model)
	if model == "" {
		return ""
	}

	normalized := strings.ReplaceAll(model, "_", "-")
	normalized = strings.Join(strings.Fields(normalized), "-")
	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}

	if strings.HasPrefix(normalized, "gpt5") {
		normalized = "gpt-5" + strings.TrimPrefix(normalized, "gpt5")
	}
	if !strings.HasPrefix(normalized, "gpt-") && !strings.Contains(normalized, "codex") {
		return ""
	}

	replacements := []struct {
		from string
		to   string
	}{
		{"gpt-5.4mini", "gpt-5.4-mini"},
		{"gpt-5.4nano", "gpt-5.4-nano"},
		{"gpt-5.3-codexspark", "gpt-5.3-codex-spark"},
		{"gpt-5.3codexspark", "gpt-5.3-codex-spark"},
		{"gpt-5.3codex", "gpt-5.3-codex"},
	}
	for _, replacement := range replacements {
		normalized = strings.ReplaceAll(normalized, replacement.from, replacement.to)
	}
	return normalized
}

// CodexBaseInstructionsForModel 按模型返回最匹配的真实 Codex base instructions：
//   - gpt-6 / gpt-6-astra（含供应商前缀与日期变体）→ GPT-6 Astra prompt
//   - 含 "codex" 的模型（gpt-5-codex / gpt-5.x-codex / codex-max / spark 等）→ GPT-5-Codex prompt
//   - gpt-5.5 系非 codex 模型 → GPT-5.5 prompt
//   - gpt-5.2 系非 codex 模型 → GPT-5.2 prompt
//   - gpt-5.1 系非 codex 模型 → GPT-5.1 prompt
//   - 其它（含 gpt-5.3 / gpt-5.4 / 裸 gpt-5 / 未知模型）→ 回退到最新版本（当前 GPT-5.5）
//
// 任一专用 prompt 意外为空时回退链最终落到 DefaultInstructions，保证返回非空。
func CodexBaseInstructionsForModel(model string) string {
	canonical := CanonicalizeOpenAIModelAliasSpelling(model)
	switch {
	case IsGPT61SolModelSpelling(canonical):
		var metadata struct {
			ModelMessages struct {
				InstructionsTemplate string `json:"instructions_template"`
			} `json:"model_messages"`
		}
		if err := json.Unmarshal(CodexGPT61SolMetadata, &metadata); err != nil {
			panic(err)
		}
		return metadata.ModelMessages.InstructionsTemplate
	case canonical == "gpt-6" || canonical == "gpt-6-astra" || strings.HasPrefix(canonical, "gpt-6-astra-"):
		if v := strings.TrimSpace(instructionsGPT6Astra); v != "" {
			return instructionsGPT6Astra
		}
	case strings.Contains(canonical, "codex"):
		return DefaultInstructions
	case strings.HasPrefix(canonical, "gpt-5.5"):
		return latestCodexInstructions()
	case strings.HasPrefix(canonical, "gpt-5.2"):
		if v := strings.TrimSpace(instructionsGPT52); v != "" {
			return instructionsGPT52
		}
	case strings.HasPrefix(canonical, "gpt-5.1"):
		if v := strings.TrimSpace(instructionsGPT51); v != "" {
			return instructionsGPT51
		}
	}
	return latestCodexInstructions()
}

// IsGPT6SolOrLunaModelSpelling 识别官方模型及支持的推理或压缩后缀。
func IsGPT6SolOrLunaModelSpelling(model string) bool {
	canonical := CanonicalizeOpenAIModelAliasSpelling(model)
	for _, base := range []string{"gpt-6-sol", "gpt-6-luna"} {
		if canonical == base {
			return true
		}
		suffix, ok := strings.CutPrefix(canonical, base+"-")
		if ok {
			switch suffix {
			case "none", "low", "medium", "high", "xhigh", "max", "openai-compact":
				return true
			}
		}
	}
	return false
}

// IsGPT61SolModelSpelling 识别 GPT-6.1 Sol 及本地推理后缀，保留非法档位供请求校验。
func IsGPT61SolModelSpelling(model string) bool {
	canonical := CanonicalizeOpenAIModelAliasSpelling(model)
	if canonical == "gpt-6.1-sol" {
		return true
	}
	suffix, ok := strings.CutPrefix(canonical, "gpt-6.1-sol-")
	if !ok {
		return false
	}
	switch suffix {
	case "none", "minimal", "low", "medium", "high", "xhigh", "max", "openai-compact":
		return true
	default:
		return false
	}
}

func IsGPT61TerraModelSpelling(model string) bool {
	canonical := CanonicalizeOpenAIModelAliasSpelling(model)
	if canonical == "gpt-6.1-terra" {
		return true
	}
	suffix, ok := strings.CutPrefix(canonical, "gpt-6.1-terra-")
	if !ok {
		return false
	}
	switch suffix {
	case "none", "low", "medium", "high", "xhigh", "max", "openai-compact":
		return true
	default:
		return false
	}
}

func RequiresResponsesForToolCalls(model string) bool {
	return IsGPT61SolModelSpelling(model) || IsGPT61TerraModelSpelling(model)
}

// ValidateGPT61SolReasoningEffort 拒绝不支持的关闭推理档位，不能静默提高客户端的推理强度。
func ValidateGPT61SolReasoningEffort(model, effort string) error {
	if IsGPT61SolModelSpelling(model) {
		switch strings.ToLower(strings.TrimSpace(effort)) {
		case "none", "minimal":
			return fmt.Errorf("gpt-6.1-sol does not support reasoning effort %q; use low, medium, high, xhigh or max", effort)
		}
	}
	return nil
}
