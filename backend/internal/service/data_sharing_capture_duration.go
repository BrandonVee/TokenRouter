package service

import (
	"sort"
	"sync"
	"time"
)

const (
	defaultDataSharingCaptureDurationWindowSize = 512
	minDataSharingCaptureDurationWindowSize     = 32
	maxDataSharingCaptureDurationWindowSize     = 10000
)

// DataShareCaptureDurationPartKey 标识采集链路中的一个耗时阶段。
type DataShareCaptureDurationPartKey string

const (
	DataShareCaptureDurationPartCaptureQueueWait  DataShareCaptureDurationPartKey = "capture_queue_wait"
	DataShareCaptureDurationPartCaptureBuild      DataShareCaptureDurationPartKey = "capture_build"
	DataShareCaptureDurationPartBufferHydrate     DataShareCaptureDurationPartKey = "buffer_hydrate"
	DataShareCaptureDurationPartBufferMerge       DataShareCaptureDurationPartKey = "buffer_merge"
	DataShareCaptureDurationPartBufferSubmitTotal DataShareCaptureDurationPartKey = "buffer_submit_total"
	DataShareCaptureDurationPartFlushQueueWait    DataShareCaptureDurationPartKey = "flush_queue_wait"
	DataShareCaptureDurationPartFlushFinalize     DataShareCaptureDurationPartKey = "flush_finalize"
	DataShareCaptureDurationPartPayloadEncode     DataShareCaptureDurationPartKey = "payload_encode"
	DataShareCaptureDurationPartStorageLimitCheck DataShareCaptureDurationPartKey = "storage_limit_check"
	DataShareCaptureDurationPartDBLookup          DataShareCaptureDurationPartKey = "db_lookup"
	DataShareCaptureDurationPartDBWrite           DataShareCaptureDurationPartKey = "db_write"
	DataShareCaptureDurationPartFlushTotal        DataShareCaptureDurationPartKey = "flush_total"
)

// DataShareCaptureDurationRecorder 接收采集链路耗时样本。
type DataShareCaptureDurationRecorder interface {
	Observe(part DataShareCaptureDurationPartKey, duration time.Duration)
}

// DataShareCaptureDurationObserveFunc 允许 repository 通过回调上报内部阶段耗时。
type DataShareCaptureDurationObserveFunc func(part DataShareCaptureDurationPartKey, duration time.Duration)

// Observe 实现 DataShareCaptureDurationRecorder，便于函数直接作为 recorder 使用。
func (f DataShareCaptureDurationObserveFunc) Observe(part DataShareCaptureDurationPartKey, duration time.Duration) {
	if f == nil {
		return
	}
	f(part, duration)
}

// DataShareCaptureDurationStats 是管理端可见的采集耗时统计快照。
type DataShareCaptureDurationStats struct {
	WindowSize  int                            `json:"window_size"`
	SampleCount int                            `json:"sample_count"`
	Parts       []DataShareCaptureDurationPart `json:"parts"`
}

// DataShareCaptureDurationPart 是单个阶段的耗时统计。
type DataShareCaptureDurationPart struct {
	Key         string                           `json:"key"`
	Label       string                           `json:"label"`
	Category    string                           `json:"category"`
	LastMillis  int64                            `json:"last_millis"`
	AvgMillis   float64                          `json:"avg_millis"`
	P50Millis   int64                            `json:"p50_millis"`
	P95Millis   int64                            `json:"p95_millis"`
	MaxMillis   int64                            `json:"max_millis"`
	SampleCount int                              `json:"sample_count"`
	Buckets     []DataShareCaptureDurationBucket `json:"buckets"`
}

// DataShareCaptureDurationBucket 是固定耗时区间的样本数量。
type DataShareCaptureDurationBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type dataShareCaptureDurationPartDefinition struct {
	key      DataShareCaptureDurationPartKey
	label    string
	category string
}

type dataShareCaptureDurationBucketDefinition struct {
	label string
	upper int64
}

var dataShareCaptureDurationPartDefinitions = []dataShareCaptureDurationPartDefinition{
	{key: DataShareCaptureDurationPartCaptureQueueWait, label: "采集排队", category: "采集"},
	{key: DataShareCaptureDurationPartCaptureBuild, label: "采集解析", category: "采集"},
	{key: DataShareCaptureDurationPartBufferHydrate, label: "冷数据读取", category: "缓冲"},
	{key: DataShareCaptureDurationPartBufferMerge, label: "缓冲合并", category: "缓冲"},
	{key: DataShareCaptureDurationPartBufferSubmitTotal, label: "入缓冲总耗时", category: "缓冲"},
	{key: DataShareCaptureDurationPartFlushQueueWait, label: "Flush 排队", category: "Flush"},
	{key: DataShareCaptureDurationPartFlushFinalize, label: "快照最终化", category: "Flush"},
	{key: DataShareCaptureDurationPartPayloadEncode, label: "Payload 编码压缩", category: "落库"},
	{key: DataShareCaptureDurationPartStorageLimitCheck, label: "容量检查", category: "落库"},
	{key: DataShareCaptureDurationPartDBLookup, label: "DB 查重", category: "落库"},
	{key: DataShareCaptureDurationPartDBWrite, label: "DB 写入", category: "落库"},
	{key: DataShareCaptureDurationPartFlushTotal, label: "Flush 总耗时", category: "Flush"},
}

var dataShareCaptureDurationBucketDefinitions = []dataShareCaptureDurationBucketDefinition{
	{label: "<10ms", upper: 10},
	{label: "10-50ms", upper: 50},
	{label: "50-100ms", upper: 100},
	{label: "100-250ms", upper: 250},
	{label: "250-500ms", upper: 500},
	{label: "0.5-1s", upper: 1000},
	{label: "1-2s", upper: 2000},
	{label: "2-5s", upper: 5000},
	{label: "5-10s", upper: 10000},
	{label: "10-30s", upper: 30000},
	{label: ">=30s", upper: -1},
}

// DataShareCaptureDurationWindowBounds 返回管理端可配置窗口的边界。
func DataShareCaptureDurationWindowBounds() (int, int, int) {
	return minDataSharingCaptureDurationWindowSize, maxDataSharingCaptureDurationWindowSize, defaultDataSharingCaptureDurationWindowSize
}

func normalizeDataShareCaptureDurationWindowSize(size int) int {
	if size <= 0 {
		size = defaultDataSharingCaptureDurationWindowSize
	}
	if size < minDataSharingCaptureDurationWindowSize {
		return minDataSharingCaptureDurationWindowSize
	}
	if size > maxDataSharingCaptureDurationWindowSize {
		return maxDataSharingCaptureDurationWindowSize
	}
	return size
}

// dataShareCaptureDurationRecorder 在进程内按阶段保存最近 N 个耗时样本。
type dataShareCaptureDurationRecorder struct {
	mu         sync.RWMutex
	windowSize int
	parts      map[DataShareCaptureDurationPartKey]*dataShareCaptureDurationRing
}

func newDataShareCaptureDurationRecorder(windowSize int) *dataShareCaptureDurationRecorder {
	recorder := &dataShareCaptureDurationRecorder{
		windowSize: normalizeDataShareCaptureDurationWindowSize(windowSize),
		parts:      map[DataShareCaptureDurationPartKey]*dataShareCaptureDurationRing{},
	}
	for _, def := range dataShareCaptureDurationPartDefinitions {
		recorder.parts[def.key] = newDataShareCaptureDurationRing(recorder.windowSize)
	}
	return recorder
}

func (r *dataShareCaptureDurationRecorder) Observe(part DataShareCaptureDurationPartKey, duration time.Duration) {
	if r == nil || part == "" {
		return
	}
	millis := duration.Milliseconds()
	if millis < 0 {
		millis = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ring := r.parts[part]
	if ring == nil {
		ring = newDataShareCaptureDurationRing(r.windowSize)
		r.parts[part] = ring
	}
	ring.add(millis)
}

func (r *dataShareCaptureDurationRecorder) SetWindowSize(windowSize int) {
	if r == nil {
		return
	}
	windowSize = normalizeDataShareCaptureDurationWindowSize(windowSize)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.windowSize == windowSize {
		return
	}
	r.windowSize = windowSize
	for _, ring := range r.parts {
		ring.resize(windowSize)
	}
}

func (r *dataShareCaptureDurationRecorder) Snapshot() DataShareCaptureDurationStats {
	if r == nil {
		return DataShareCaptureDurationStats{WindowSize: defaultDataSharingCaptureDurationWindowSize}
	}
	r.mu.RLock()
	windowSize := r.windowSize
	samplesByPart := make(map[DataShareCaptureDurationPartKey][]int64, len(r.parts))
	for key, ring := range r.parts {
		samplesByPart[key] = ring.samples()
	}
	r.mu.RUnlock()

	stats := DataShareCaptureDurationStats{
		WindowSize: windowSize,
		Parts:      make([]DataShareCaptureDurationPart, 0, len(dataShareCaptureDurationPartDefinitions)),
	}
	for _, def := range dataShareCaptureDurationPartDefinitions {
		part := buildDataShareCaptureDurationPart(def, samplesByPart[def.key])
		if part.SampleCount > stats.SampleCount {
			stats.SampleCount = part.SampleCount
		}
		stats.Parts = append(stats.Parts, part)
	}
	return stats
}

func buildDataShareCaptureDurationPart(def dataShareCaptureDurationPartDefinition, samples []int64) DataShareCaptureDurationPart {
	part := DataShareCaptureDurationPart{
		Key:      string(def.key),
		Label:    def.label,
		Category: def.category,
		Buckets:  buildDataShareCaptureDurationBuckets(samples),
	}
	if len(samples) == 0 {
		return part
	}
	part.SampleCount = len(samples)
	part.LastMillis = samples[len(samples)-1]
	sorted := append([]int64(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	var total int64
	for _, value := range samples {
		total += value
		if value > part.MaxMillis {
			part.MaxMillis = value
		}
	}
	part.AvgMillis = float64(total) / float64(len(samples))
	part.P50Millis = percentileFromSortedMillis(sorted, 0.50)
	part.P95Millis = percentileFromSortedMillis(sorted, 0.95)
	return part
}

func buildDataShareCaptureDurationBuckets(samples []int64) []DataShareCaptureDurationBucket {
	buckets := make([]DataShareCaptureDurationBucket, len(dataShareCaptureDurationBucketDefinitions))
	for i, def := range dataShareCaptureDurationBucketDefinitions {
		buckets[i] = DataShareCaptureDurationBucket{Label: def.label}
	}
	for _, sample := range samples {
		for i, def := range dataShareCaptureDurationBucketDefinitions {
			if def.upper < 0 || sample < def.upper {
				buckets[i].Count++
				break
			}
		}
	}
	return buckets
}

func percentileFromSortedMillis(sorted []int64, percentile float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sorted[0]
	}
	if percentile >= 1 {
		return sorted[len(sorted)-1]
	}
	index := int(percentile*float64(len(sorted)-1) + 0.5)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

type dataShareCaptureDurationRing struct {
	values []int64
	next   int
	count  int
}

func newDataShareCaptureDurationRing(size int) *dataShareCaptureDurationRing {
	size = normalizeDataShareCaptureDurationWindowSize(size)
	return &dataShareCaptureDurationRing{values: make([]int64, size)}
}

func (r *dataShareCaptureDurationRing) add(value int64) {
	if r == nil || len(r.values) == 0 {
		return
	}
	r.values[r.next] = value
	r.next = (r.next + 1) % len(r.values)
	if r.count < len(r.values) {
		r.count++
	}
}

func (r *dataShareCaptureDurationRing) samples() []int64 {
	if r == nil || r.count == 0 {
		return nil
	}
	out := make([]int64, 0, r.count)
	start := 0
	if r.count == len(r.values) {
		start = r.next
	}
	for i := 0; i < r.count; i++ {
		out = append(out, r.values[(start+i)%len(r.values)])
	}
	return out
}

func (r *dataShareCaptureDurationRing) resize(size int) {
	if r == nil {
		return
	}
	size = normalizeDataShareCaptureDurationWindowSize(size)
	samples := r.samples()
	if len(samples) > size {
		samples = samples[len(samples)-size:]
	}
	r.values = make([]int64, size)
	r.next = 0
	r.count = 0
	for _, sample := range samples {
		r.add(sample)
	}
}
