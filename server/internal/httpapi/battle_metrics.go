package httpapi

import (
	"math"
	"sort"
	"sync"
	"time"
)

const battleMetricsWindowSize = 2048

type battleMetricWindow[T any] struct {
	buf  []T
	next int
	size int
}

func (w *battleMetricWindow[T]) add(sample T) {
	if battleMetricsWindowSize <= 0 {
		return
	}
	if len(w.buf) == 0 {
		w.buf = make([]T, battleMetricsWindowSize)
	}
	w.buf[w.next] = sample
	w.next++
	if w.next == len(w.buf) {
		w.next = 0
	}
	if w.size < len(w.buf) {
		w.size++
	}
}

func (w *battleMetricWindow[T]) orderedCopy() []T {
	if w == nil || w.size == 0 {
		return nil
	}
	out := make([]T, w.size)
	start := w.next - w.size
	if start < 0 {
		start += len(w.buf)
	}
	copied := copy(out, w.buf[start:])
	copy(out[copied:], w.buf[:w.size-copied])
	return out
}

type battleMetricsCollector struct {
	mu sync.Mutex

	updatedAt                     time.Time
	batches                       battleMetricWindow[battleBatchMetricSample]
	compiles                      battleMetricWindow[battleStageMetricSample]
	playbacks                     battleMetricWindow[battleStageMetricSample]
	playbackDispatches            battleMetricWindow[battlePlaybackDispatchMetricSample]
	roomDetails                   battleMetricWindow[battleRoomDetailMetricSample]
	wsBattleBatch                 battleWSBatchDeliveryCounters
	settlementRejectedNotFinished int64
	playbackWorkerRestarts        int64
	playbackWorkerRecoveries      int64
}

type battleBatchMetricSample struct {
	Elapsed       time.Duration
	ClaimElapsed  time.Duration
	ExecElapsed   time.Duration
	LeaseUtilPct  float64
	Rooms         int
	CompiledRuns  int64
	CompiledTurns int64
}

type battleStageMetricSample struct {
	Elapsed     time.Duration
	Turns       int64
	TimedOut    bool
	StepLimited bool
}

type battlePlaybackDispatchMetricSample struct {
	DispatchInterval time.Duration
	BaseInterval     time.Duration
	OffsetGap        time.Duration
	BatchSpan        time.Duration
	NextDelay        time.Duration
	Events           int
}

type battleRoomDetailMetricSample struct {
	Elapsed  time.Duration
	CacheHit bool
}

type battleWSBatchDeliveryCounters struct {
	Attempts          int64 `json:"attempts"`
	Delivered         int64 `json:"delivered"`
	SkippedPresence   int64 `json:"skippedPresence"`
	SkippedSubscribed int64 `json:"skippedNotSubscribed"`
}

type battleMetricsSnapshot struct {
	UpdatedAtMS                   int64                              `json:"updatedAtMs"`
	WindowSize                    int                                `json:"windowSize"`
	Batch                         battleBatchMetricsStats            `json:"batch"`
	Compile                       battleStageMetricsStats            `json:"compile"`
	Playback                      battleStageMetricsStats            `json:"playback"`
	PlaybackDispatch              battlePlaybackDispatchMetricsStats `json:"playbackDispatch"`
	RoomDetail                    battleRoomDetailMetricsStats       `json:"roomDetail"`
	WSBatchDelivery               battleWSBatchDeliveryCounters      `json:"wsBatchDelivery"`
	SettlementRejectedNotFinished int64                              `json:"settlementRejectedNotFinishedCount"`
	PlaybackWorkerRestarts        int64                              `json:"playbackWorkerRestarts"`
	PlaybackStuckRecoveries       int64                              `json:"playbackStuckRecoveries"`
}

type battleBatchMetricsStats struct {
	Samples          int              `json:"samples"`
	ElapsedMS        battleValueStats `json:"elapsedMs"`
	ClaimElapsedMS   battleValueStats `json:"claimElapsedMs"`
	ExecElapsedMS    battleValueStats `json:"execElapsedMs"`
	LeaseUtilization battleValueStats `json:"leaseUtilizationPct"`
	Rooms            battleValueStats `json:"rooms"`
	CompiledRuns     int64            `json:"compiledRuns"`
	CompiledTurns    int64            `json:"compiledTurns"`
}

type battleStageMetricsStats struct {
	Samples     int              `json:"samples"`
	ElapsedMS   battleValueStats `json:"elapsedMs"`
	Turns       battleValueStats `json:"turns,omitempty"`
	TimedOut    int64            `json:"timedOut,omitempty"`
	StepLimited int64            `json:"stepLimited,omitempty"`
}

type battlePlaybackDispatchMetricsStats struct {
	Samples            int              `json:"samples"`
	DispatchIntervalMS battleValueStats `json:"dispatchIntervalMs"`
	BaseIntervalMS     battleValueStats `json:"baseIntervalMs"`
	OffsetGapMS        battleValueStats `json:"offsetGapMs"`
	BatchSpanMS        battleValueStats `json:"batchSpanMs"`
	NextDelayMS        battleValueStats `json:"nextDelayMs"`
	Events             battleValueStats `json:"events"`
}

type battleRoomDetailMetricsStats struct {
	Samples      int              `json:"samples"`
	ElapsedMS    battleValueStats `json:"elapsedMs"`
	CacheHitRate float64          `json:"cacheHitRate"`
}

type battleValueStats struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
}

func (c *battleMetricsCollector) AddBatch(sample battleBatchMetricSample) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.batches.add(sample)
}

func (c *battleMetricsCollector) AddCompile(sample battleStageMetricSample) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.compiles.add(sample)
}

func (c *battleMetricsCollector) AddPlayback(sample battleStageMetricSample) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.playbacks.add(sample)
}

func (c *battleMetricsCollector) AddPlaybackDispatch(sample battlePlaybackDispatchMetricSample) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.playbackDispatches.add(sample)
}

func (c *battleMetricsCollector) AddRoomDetail(elapsed time.Duration, cacheHit bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.roomDetails.add(battleRoomDetailMetricSample{Elapsed: elapsed, CacheHit: cacheHit})
}

func (c *battleMetricsCollector) AddPlaybackWorkerRestart() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.playbackWorkerRestarts++
}

func (c *battleMetricsCollector) AddPlaybackStuckRecovery() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.playbackWorkerRecoveries++
}

func (c *battleMetricsCollector) AddWSBattleBatchDelivery(attempts, delivered, skippedPresence, skippedSubscribed int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.wsBattleBatch.Attempts += int64(maxIntFloor(attempts, 0))
	c.wsBattleBatch.Delivered += int64(maxIntFloor(delivered, 0))
	c.wsBattleBatch.SkippedPresence += int64(maxIntFloor(skippedPresence, 0))
	c.wsBattleBatch.SkippedSubscribed += int64(maxIntFloor(skippedSubscribed, 0))
}

func (c *battleMetricsCollector) AddSettlementRejectedNotFinished() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updatedAt = time.Now()
	c.settlementRejectedNotFinished++
}

func (c *battleMetricsCollector) Snapshot() battleMetricsSnapshot {
	if c == nil {
		return battleMetricsSnapshot{WindowSize: battleMetricsWindowSize}
	}
	c.mu.Lock()
	updatedAt := c.updatedAt
	batches := c.batches.orderedCopy()
	compiles := c.compiles.orderedCopy()
	playbacks := c.playbacks.orderedCopy()
	playbackDispatches := c.playbackDispatches.orderedCopy()
	roomDetails := c.roomDetails.orderedCopy()
	wsBattleBatch := c.wsBattleBatch
	settlementRejectedNotFinished := c.settlementRejectedNotFinished
	playbackWorkerRestarts := c.playbackWorkerRestarts
	playbackWorkerRecoveries := c.playbackWorkerRecoveries
	c.mu.Unlock()
	return battleMetricsSnapshot{
		UpdatedAtMS:                   updatedAt.UnixMilli(),
		WindowSize:                    battleMetricsWindowSize,
		Batch:                         summarizeBattleBatchSamples(batches),
		Compile:                       summarizeBattleStageSamples(compiles),
		Playback:                      summarizeBattleStageSamples(playbacks),
		PlaybackDispatch:              summarizeBattlePlaybackDispatchSamples(playbackDispatches),
		RoomDetail:                    summarizeBattleRoomDetailSamples(roomDetails),
		WSBatchDelivery:               wsBattleBatch,
		SettlementRejectedNotFinished: settlementRejectedNotFinished,
		PlaybackWorkerRestarts:        playbackWorkerRestarts,
		PlaybackStuckRecoveries:       playbackWorkerRecoveries,
	}
}

func summarizeBattleRoomDetailSamples(samples []battleRoomDetailMetricSample) battleRoomDetailMetricsStats {
	if len(samples) == 0 {
		return battleRoomDetailMetricsStats{}
	}
	elapsed := make([]float64, 0, len(samples))
	cacheHits := 0
	for _, sample := range samples {
		elapsed = append(elapsed, battleDurationToMS(sample.Elapsed))
		if sample.CacheHit {
			cacheHits++
		}
	}
	return battleRoomDetailMetricsStats{
		Samples:      len(samples),
		ElapsedMS:    summarizeBattleValues(elapsed),
		CacheHitRate: float64(cacheHits) / float64(len(samples)),
	}
}

func summarizeBattleBatchSamples(samples []battleBatchMetricSample) battleBatchMetricsStats {
	if len(samples) == 0 {
		return battleBatchMetricsStats{}
	}
	elapsed := make([]float64, 0, len(samples))
	claim := make([]float64, 0, len(samples))
	exec := make([]float64, 0, len(samples))
	util := make([]float64, 0, len(samples))
	rooms := make([]float64, 0, len(samples))
	var compiledRuns int64
	var compiledTurns int64
	for _, sample := range samples {
		elapsed = append(elapsed, battleDurationToMS(sample.Elapsed))
		claim = append(claim, battleDurationToMS(sample.ClaimElapsed))
		exec = append(exec, battleDurationToMS(sample.ExecElapsed))
		util = append(util, sample.LeaseUtilPct)
		rooms = append(rooms, float64(sample.Rooms))
		compiledRuns += sample.CompiledRuns
		compiledTurns += sample.CompiledTurns
	}
	return battleBatchMetricsStats{
		Samples:          len(samples),
		ElapsedMS:        summarizeBattleValues(elapsed),
		ClaimElapsedMS:   summarizeBattleValues(claim),
		ExecElapsedMS:    summarizeBattleValues(exec),
		LeaseUtilization: summarizeBattleValues(util),
		Rooms:            summarizeBattleValues(rooms),
		CompiledRuns:     compiledRuns,
		CompiledTurns:    compiledTurns,
	}
}

func summarizeBattleStageSamples(samples []battleStageMetricSample) battleStageMetricsStats {
	if len(samples) == 0 {
		return battleStageMetricsStats{}
	}
	elapsed := make([]float64, 0, len(samples))
	turns := make([]float64, 0, len(samples))
	var timedOut, stepLimited int64
	for _, sample := range samples {
		elapsed = append(elapsed, battleDurationToMS(sample.Elapsed))
		turns = append(turns, float64(sample.Turns))
		if sample.TimedOut {
			timedOut++
		}
		if sample.StepLimited {
			stepLimited++
		}
	}
	return battleStageMetricsStats{
		Samples:     len(samples),
		ElapsedMS:   summarizeBattleValues(elapsed),
		Turns:       summarizeBattleValues(turns),
		TimedOut:    timedOut,
		StepLimited: stepLimited,
	}
}

func summarizeBattlePlaybackDispatchSamples(samples []battlePlaybackDispatchMetricSample) battlePlaybackDispatchMetricsStats {
	if len(samples) == 0 {
		return battlePlaybackDispatchMetricsStats{}
	}
	dispatchInterval := make([]float64, 0, len(samples))
	baseInterval := make([]float64, 0, len(samples))
	offsetGap := make([]float64, 0, len(samples))
	batchSpan := make([]float64, 0, len(samples))
	nextDelay := make([]float64, 0, len(samples))
	events := make([]float64, 0, len(samples))
	for _, sample := range samples {
		dispatchInterval = append(dispatchInterval, battleDurationToMS(sample.DispatchInterval))
		baseInterval = append(baseInterval, battleDurationToMS(sample.BaseInterval))
		offsetGap = append(offsetGap, battleDurationToMS(sample.OffsetGap))
		batchSpan = append(batchSpan, battleDurationToMS(sample.BatchSpan))
		nextDelay = append(nextDelay, battleDurationToMS(sample.NextDelay))
		events = append(events, float64(sample.Events))
	}
	return battlePlaybackDispatchMetricsStats{
		Samples:            len(samples),
		DispatchIntervalMS: summarizeBattleValues(dispatchInterval),
		BaseIntervalMS:     summarizeBattleValues(baseInterval),
		OffsetGapMS:        summarizeBattleValues(offsetGap),
		BatchSpanMS:        summarizeBattleValues(batchSpan),
		NextDelayMS:        summarizeBattleValues(nextDelay),
		Events:             summarizeBattleValues(events),
	}
}

func summarizeBattleValues(values []float64) battleValueStats {
	if len(values) == 0 {
		return battleValueStats{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	total := 0.0
	for _, value := range sorted {
		total += value
	}
	return battleValueStats{
		Count: len(sorted),
		Min:   sorted[0],
		P50:   percentileFloat64(sorted, 50),
		P95:   percentileFloat64(sorted, 95),
		P99:   percentileFloat64(sorted, 99),
		Max:   sorted[len(sorted)-1],
		Avg:   math.Round((total/float64(len(sorted)))*1000) / 1000,
	}
}

func percentileFloat64(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return values[0]
	}
	if percentile >= 100 {
		return values[len(values)-1]
	}
	rank := int(math.Ceil((percentile / 100) * float64(len(values))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(values) {
		rank = len(values)
	}
	return values[rank-1]
}

func battleDurationToMS(d time.Duration) float64 {
	return math.Round((float64(d)/float64(time.Millisecond))*1000) / 1000
}

func maxIntFloor(value int, floor int) int {
	if value < floor {
		return floor
	}
	return value
}
