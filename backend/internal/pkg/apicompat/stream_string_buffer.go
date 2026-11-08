// 上游兼容适配：协议形状、用量或流状态边界的实现与回归。
package apicompat

import "strings"

func appendStreamString(buffer **strings.Builder, current, delta string) string {
	if delta == "" {
		return current
	}
	if *buffer == nil || (*buffer).String() != current {
		*buffer = &strings.Builder{}
		_, _ = (*buffer).WriteString(current)
	}
	_, _ = (*buffer).WriteString(delta)
	return (*buffer).String()
}

func appendIndexedStreamString(buffers *map[int]*strings.Builder, index int, current, delta string) string {
	if *buffers == nil {
		*buffers = make(map[int]*strings.Builder)
	}
	buffer := (*buffers)[index]
	value := appendStreamString(&buffer, current, delta)
	(*buffers)[index] = buffer
	return value
}

type streamStringBytes struct {
	data     []byte
	snapshot string
}

func appendStreamStringBytes(buffer **streamStringBytes, previous, current, delta string) []byte {
	if *buffer == nil || (*buffer).snapshot != previous {
		*buffer = &streamStringBytes{data: append([]byte(nil), previous...)}
	}
	(*buffer).data = append((*buffer).data, delta...)
	(*buffer).snapshot = current
	return (*buffer).data
}
