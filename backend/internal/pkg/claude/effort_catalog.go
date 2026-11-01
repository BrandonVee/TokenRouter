package claude

import (
	"strings"
	"unicode"
)

var (
	effortLowMediumHigh         = []string{"low", "medium", "high"}
	effortLowMediumHighMax      = []string{"low", "medium", "high", "max"}
	effortLowMediumHighXHighMax = []string{"low", "medium", "high", "xhigh", "max"}
)

var effortFamilies = []struct {
	family string
	levels []string
}{
	{family: "claude-mythos-preview", levels: effortLowMediumHighMax},
	{family: "claude-mythos-5", levels: effortLowMediumHighXHighMax},
	{family: "claude-fable-5", levels: effortLowMediumHighXHighMax},
	{family: "claude-sonnet-4-6", levels: effortLowMediumHighMax},
	{family: "claude-sonnet-5-5", levels: effortLowMediumHighXHighMax},
	{family: "claude-sonnet-5", levels: effortLowMediumHighXHighMax},
	{family: "claude-opus-4-8", levels: effortLowMediumHighXHighMax},
	{family: "claude-opus-4-7", levels: effortLowMediumHighXHighMax},
	{family: "claude-opus-4-6", levels: effortLowMediumHighMax},
	{family: "claude-opus-4-5", levels: effortLowMediumHigh},
	{family: "claude-opus-5-5", levels: effortLowMediumHighXHighMax},
	{family: "claude-opus-5", levels: effortLowMediumHighXHighMax},
	{family: "claude-haiku-5-5", levels: effortLowMediumHighXHighMax},
}

// EffortLevelsForModel 返回模型接受的推理档位，按从轻到深排序。
func EffortLevelsForModel(model string) []string {
	id := normalizeEffortModelID(model)
	for _, entry := range effortFamilies {
		if id == entry.family || strings.HasPrefix(id, entry.family+"-") {
			return append([]string(nil), entry.levels...)
		}
	}
	return nil
}

// IsOpus55 规范化提供商与本地后缀后识别 Opus 5.5。
func IsOpus55(model string) bool {
	return normalizeEffortModelID(model) == "claude-opus-5-5"
}

// IsSonnet55 规范化提供商与本地后缀后识别 Sonnet 5.5。
func IsSonnet55(model string) bool {
	return normalizeEffortModelID(model) == "claude-sonnet-5-5"
}

// IsHaiku55 规范化提供商与本地后缀后识别 Haiku 5.5。
func IsHaiku55(model string) bool {
	return normalizeEffortModelID(model) == "claude-haiku-5-5"
}

func normalizeEffortModelID(model string) string {
	id := strings.ToLower(strings.TrimSpace(model))
	id = strings.TrimPrefix(id, "models/")
	if slash := strings.IndexByte(id, '/'); slash >= 0 {
		id = strings.TrimPrefix(strings.TrimSpace(id[slash+1:]), "models/")
	}
	for _, prefix := range []string{"us.", "eu.", "apac.", "jp.", "au.", "us-gov.", "global."} {
		id = strings.TrimPrefix(id, prefix)
	}
	id = strings.TrimPrefix(id, "anthropic.")
	id = strings.TrimSuffix(id, "-thinking")
	// OpenRouter 的点号版本先转为规范 ID，再解析推理、思考及计费模型族。
	if id == "claude-opus-5.5" {
		id = "claude-opus-5-5"
	}
	if id == "claude-sonnet-5.5" {
		id = "claude-sonnet-5-5"
	}
	if id == "claude-haiku-5.5" {
		id = "claude-haiku-5-5"
	}
	if mapped, ok := ModelIDReverseOverrides[id]; ok {
		id = mapped
	}
	if len(id) >= 9 {
		suffix := id[len(id)-9:]
		if suffix[0] == '-' {
			digits := true
			for _, r := range suffix[1:] {
				if !unicode.IsDigit(r) {
					digits = false
					break
				}
			}
			if digits {
				id = id[:len(id)-9]
			}
		}
	}
	return id
}
