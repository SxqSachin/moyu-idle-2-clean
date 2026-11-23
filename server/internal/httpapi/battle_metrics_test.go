package httpapi

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestBattleMetricWindowZeroValueAndCopyIsolation(t *testing.T) {
	var window battleMetricWindow[int]
	if got := window.orderedCopy(); len(got) != 0 {
		t.Fatalf("empty window len=%d, want 0", len(got))
	}

	window.add(7)
	got := window.orderedCopy()
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("window=%v, want [7]", got)
	}
	got[0] = 99
	if current := window.orderedCopy(); len(current) != 1 || current[0] != 7 {
		t.Fatalf("orderedCopy aliases backing storage: %v", current)
	}
}

func TestBattleMetricWindowCapacityBoundaries(t *testing.T) {
	var window battleMetricWindow[int]
	assertValues := func(wantFirst, wantLast, wantLen int) {
		t.Helper()
		got := window.orderedCopy()
		if len(got) != wantLen {
			t.Fatalf("window len=%d, want %d", len(got), wantLen)
		}
		if wantLen == 0 {
			return
		}
		if got[0] != wantFirst || got[len(got)-1] != wantLast {
			t.Fatalf("window bounds=(%d,%d), want (%d,%d)", got[0], got[len(got)-1], wantFirst, wantLast)
		}
		for index, value := range got {
			if want := wantFirst + index; value != want {
				t.Fatalf("window[%d]=%d, want %d", index, value, want)
			}
		}
	}

	for value := 0; value < battleMetricsWindowSize-1; value++ {
		window.add(value)
	}
	assertValues(0, battleMetricsWindowSize-2, battleMetricsWindowSize-1)

	window.add(battleMetricsWindowSize - 1)
	assertValues(0, battleMetricsWindowSize-1, battleMetricsWindowSize)

	window.add(battleMetricsWindowSize)
	assertValues(1, battleMetricsWindowSize, battleMetricsWindowSize)

	for value := battleMetricsWindowSize + 1; value < 2*battleMetricsWindowSize+3; value++ {
		window.add(value)
	}
	assertValues(battleMetricsWindowSize+3, 2*battleMetricsWindowSize+2, battleMetricsWindowSize)
}

func TestBattleMetricsCollectorSnapshot(t *testing.T) {
	collector := &battleMetricsCollector{}
	collector.AddBatch(battleBatchMetricSample{Elapsed: 100 * time.Millisecond, ClaimElapsed: 10 * time.Millisecond, ExecElapsed: 90 * time.Millisecond, LeaseUtilPct: 50, Rooms: 10, CompiledRuns: 1, CompiledTurns: 12})
	collector.AddBatch(battleBatchMetricSample{Elapsed: 200 * time.Millisecond, ClaimElapsed: 20 * time.Millisecond, ExecElapsed: 180 * time.Millisecond, LeaseUtilPct: 80, Rooms: 20, CompiledRuns: 2, CompiledTurns: 34})
	collector.AddCompile(battleStageMetricSample{Elapsed: 300 * time.Millisecond, Turns: 40})
	collector.AddCompile(battleStageMetricSample{Elapsed: 500 * time.Millisecond, Turns: 60, TimedOut: true})
	collector.AddPlayback(battleStageMetricSample{Elapsed: 40 * time.Millisecond})
	collector.AddPlayback(battleStageMetricSample{Elapsed: 80 * time.Millisecond})
	collector.AddPlaybackDispatch(battlePlaybackDispatchMetricSample{
		DispatchInterval: 1800 * time.Millisecond,
		BaseInterval:     1380 * time.Millisecond,
		OffsetGap:        1800 * time.Millisecond,
		BatchSpan:        200 * time.Millisecond,
		NextDelay:        1800 * time.Millisecond,
		Events:           3,
	})
	collector.AddPlaybackDispatch(battlePlaybackDispatchMetricSample{
		DispatchInterval: 2400 * time.Millisecond,
		BaseInterval:     1380 * time.Millisecond,
		OffsetGap:        2400 * time.Millisecond,
		BatchSpan:        300 * time.Millisecond,
		NextDelay:        2400 * time.Millisecond,
		Events:           5,
	})
	collector.AddWSBattleBatchDelivery(12, 8, 2, 2)

	snapshot := collector.Snapshot()
	if snapshot.WindowSize != battleMetricsWindowSize {
		t.Fatalf("windowSize=%d, want %d", snapshot.WindowSize, battleMetricsWindowSize)
	}
	if snapshot.Batch.Samples != 2 {
		t.Fatalf("batch samples=%d, want 2", snapshot.Batch.Samples)
	}
	if snapshot.Batch.ElapsedMS.P95 < 200 || snapshot.Batch.ElapsedMS.P95 > 200 {
		t.Fatalf("batch p95=%v, want 200", snapshot.Batch.ElapsedMS.P95)
	}
	if snapshot.Batch.ClaimElapsedMS.P95 != 20 {
		t.Fatalf("claim p95=%v, want 20", snapshot.Batch.ClaimElapsedMS.P95)
	}
	if snapshot.Batch.CompiledRuns != 3 || snapshot.Batch.CompiledTurns != 46 {
		t.Fatalf("batch compiled counters = (%d,%d), want (3,46)", snapshot.Batch.CompiledRuns, snapshot.Batch.CompiledTurns)
	}
	if snapshot.Compile.Samples != 2 || snapshot.Compile.ElapsedMS.P95 != 500 {
		t.Fatalf("compile snapshot unexpected: %+v", snapshot.Compile)
	}
	if snapshot.Compile.TimedOut != 1 {
		t.Fatalf("compile timedOut=%d, want 1", snapshot.Compile.TimedOut)
	}
	if snapshot.Playback.Samples != 2 || snapshot.Playback.ElapsedMS.P95 != 80 {
		t.Fatalf("playback snapshot unexpected: %+v", snapshot.Playback)
	}
	if snapshot.PlaybackDispatch.Samples != 2 || snapshot.PlaybackDispatch.DispatchIntervalMS.P95 != 2400 {
		t.Fatalf("playback dispatch snapshot unexpected: %+v", snapshot.PlaybackDispatch)
	}
	if snapshot.WSBatchDelivery.Attempts != 12 || snapshot.WSBatchDelivery.Delivered != 8 {
		t.Fatalf("ws battle delivery snapshot unexpected: %+v", snapshot.WSBatchDelivery)
	}
	if snapshot.WSBatchDelivery.SkippedPresence != 2 || snapshot.WSBatchDelivery.SkippedSubscribed != 2 {
		t.Fatalf("ws battle delivery skip counters unexpected: %+v", snapshot.WSBatchDelivery)
	}
	if math.Abs(snapshot.Batch.LeaseUtilization.P95-80) > 0.001 {
		t.Fatalf("lease utilization p95=%v, want 80", snapshot.Batch.LeaseUtilization.P95)
	}
}

func TestBattleMetricsCollectorRetainsNewestSamples(t *testing.T) {
	collector := &battleMetricsCollector{}
	for index := 0; index < battleMetricsWindowSize+2; index++ {
		collector.AddPlaybackDispatch(battlePlaybackDispatchMetricSample{Events: index})
		compiledRuns := int64(1)
		if index < 2 {
			compiledRuns = 100_000
		}
		collector.AddBatch(battleBatchMetricSample{CompiledRuns: compiledRuns})
		collector.AddRoomDetail(time.Duration(index)*time.Millisecond, index < 2)
	}

	snapshot := collector.Snapshot()
	if snapshot.PlaybackDispatch.Samples != battleMetricsWindowSize || snapshot.PlaybackDispatch.Events.Count != battleMetricsWindowSize {
		t.Fatalf("playback dispatch sample counts unexpected: %+v", snapshot.PlaybackDispatch)
	}
	if snapshot.PlaybackDispatch.Events.Min != 2 || snapshot.PlaybackDispatch.Events.Max != float64(battleMetricsWindowSize+1) {
		t.Fatalf("playback dispatch event bounds unexpected: %+v", snapshot.PlaybackDispatch.Events)
	}
	if snapshot.Batch.Samples != battleMetricsWindowSize || snapshot.Batch.CompiledRuns != int64(battleMetricsWindowSize) {
		t.Fatalf("batch retained window unexpected: %+v", snapshot.Batch)
	}
	if snapshot.RoomDetail.Samples != battleMetricsWindowSize || snapshot.RoomDetail.CacheHitRate != 0 {
		t.Fatalf("room detail retained window unexpected: %+v", snapshot.RoomDetail)
	}
}

func TestBattleMetricsCollectorConcurrentAddAndSnapshot(t *testing.T) {
	collector := &battleMetricsCollector{}
	const writers = 4
	const samplesPerWriter = 700
	start := make(chan struct{})
	var wg sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		wg.Add(1)
		go func(writer int) {
			defer wg.Done()
			<-start
			for index := 0; index < samplesPerWriter; index++ {
				collector.AddPlaybackDispatch(battlePlaybackDispatchMetricSample{Events: writer*samplesPerWriter + index})
				collector.AddRoomDetail(time.Duration(index)*time.Microsecond, index%2 == 0)
				collector.AddWSBattleBatchDelivery(1, 1, 0, 0)
			}
		}(writer)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for index := 0; index < 64; index++ {
			_ = collector.Snapshot()
		}
	}()
	close(start)
	wg.Wait()

	snapshot := collector.Snapshot()
	if snapshot.PlaybackDispatch.Samples != battleMetricsWindowSize {
		t.Fatalf("playback dispatch samples=%d, want %d", snapshot.PlaybackDispatch.Samples, battleMetricsWindowSize)
	}
	if snapshot.RoomDetail.Samples != battleMetricsWindowSize {
		t.Fatalf("room detail samples=%d, want %d", snapshot.RoomDetail.Samples, battleMetricsWindowSize)
	}
	wantDeliveries := int64(writers * samplesPerWriter)
	if snapshot.WSBatchDelivery.Attempts != wantDeliveries || snapshot.WSBatchDelivery.Delivered != wantDeliveries {
		t.Fatalf("ws delivery counters=%+v, want attempts=delivered=%d", snapshot.WSBatchDelivery, wantDeliveries)
	}
}

func TestBattleMetricsCollectorSteadyStateAddAllocations(t *testing.T) {
	collector := &battleMetricsCollector{}
	sample := battlePlaybackDispatchMetricSample{Events: 1}
	for index := 0; index < battleMetricsWindowSize; index++ {
		collector.AddPlaybackDispatch(sample)
	}
	if allocs := testing.AllocsPerRun(1000, func() {
		collector.AddPlaybackDispatch(sample)
	}); allocs != 0 {
		t.Fatalf("steady-state allocations=%v, want 0", allocs)
	}
}

func BenchmarkBattleMetricsCollectorAddPlaybackDispatchFull(b *testing.B) {
	collector := &battleMetricsCollector{}
	sample := battlePlaybackDispatchMetricSample{Events: 1}
	for index := 0; index < battleMetricsWindowSize; index++ {
		collector.AddPlaybackDispatch(sample)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		collector.AddPlaybackDispatch(sample)
	}
}
