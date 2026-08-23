package middleware

import (
	"strings"
	"unicode/utf8"
)

const (
	maxPersistentRequestIDBytes = 64
	maxPersistentUserAgentBytes = 512
)

// normalizePersistentText 在攻击者可控元数据进入日志或数据库列前限制其大小，并保留有效 UTF-8 内容。
func normalizePersistentText(value string, maxBytes int) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, ""))
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func normalizeCorrelationID(value string) (string, bool) {
	value = strings.TrimSpace(strings.ToValidUTF8(value, ""))
	return value, value != "" && len(value) <= maxPersistentRequestIDBytes
}
