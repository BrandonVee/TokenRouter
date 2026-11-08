// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package gemini

import (
	"strconv"
	"strings"
)

func SupportsMultimodalFunctionResponses(model string) bool {
	model = strings.TrimPrefix(model, "models/")
	version, ok := strings.CutPrefix(model, "gemini-")
	if !ok {
		return false
	}
	major, _, _ := strings.Cut(version, "-")
	major, _, _ = strings.Cut(major, ".")
	n, err := strconv.Atoi(major)
	return err == nil && n >= 3
}
