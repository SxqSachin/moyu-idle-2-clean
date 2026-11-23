package httpapi

import "testing"

func TestShouldArchiveBattleSlowRun(t *testing.T) {
	tests := []struct {
		name              string
		queueSinceStartMs int64
		compileElapsedMs  int64
		compiled          battleRunCompileResult
		want              bool
	}{
		{name: "normal run", queueSinceStartMs: 10, compileElapsedMs: 20, compiled: battleRunCompileResult{}, want: false},
		{name: "queue slow", queueSinceStartMs: battleCompileQueueSlowLogThresholdMs, compileElapsedMs: 20, compiled: battleRunCompileResult{}, want: true},
		{name: "compile slow", queueSinceStartMs: 10, compileElapsedMs: battleCompileSlowLogThreshold.Milliseconds(), compiled: battleRunCompileResult{}, want: true},
		{name: "timed out", queueSinceStartMs: 10, compileElapsedMs: 20, compiled: battleRunCompileResult{TimedOut: true}, want: true},
		{name: "step limited", queueSinceStartMs: 10, compileElapsedMs: 20, compiled: battleRunCompileResult{StepLimited: true}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldArchiveBattleSlowRun(tc.queueSinceStartMs, tc.compileElapsedMs, tc.compiled); got != tc.want {
				t.Fatalf("shouldArchiveBattleSlowRun()=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestBattleSlowRunAnalysisFromSummary(t *testing.T) {
	analysis := battleSlowRunAnalysisFromSummary(
		map[string]any{"queueSlow": true, "compileSlow": false},
		map[string]any{"preActionMs": int64(5), "executeMs": int64(12), "postActionMs": int64(3)},
		false,
		false,
	)
	if analysis.PrimaryCause != "queue_backlog" {
		t.Fatalf("primaryCause=%q, want queue_backlog", analysis.PrimaryCause)
	}
	if analysis.DominantTurnPhase != "execute" || analysis.DominantTurnPhaseMs != 12 {
		t.Fatalf("dominant phase = %q/%d, want execute/12", analysis.DominantTurnPhase, analysis.DominantTurnPhaseMs)
	}

	analysis = battleSlowRunAnalysisFromSummary(
		map[string]any{"queueSlow": true, "compileSlow": true},
		map[string]any{"preActionMs": int64(8), "executeMs": int64(7), "postActionMs": int64(9)},
		false,
		true,
	)
	if analysis.PrimaryCause != "mixed" {
		t.Fatalf("primaryCause=%q, want mixed", analysis.PrimaryCause)
	}
	if analysis.DominantTurnPhase != "postAction" || analysis.DominantTurnPhaseMs != 9 {
		t.Fatalf("dominant phase = %q/%d, want postAction/9", analysis.DominantTurnPhase, analysis.DominantTurnPhaseMs)
	}
}

func TestBattlePersistableFinalResultRemovesPlaybackKeyframes(t *testing.T) {
	in := map[string]any{
		"outcome":           "won",
		"playbackKeyframes": []map[string]any{{"nextEventIdx": 1}},
		"totalEvents":       12,
	}
	out := battlePersistableFinalResult(in)
	if _, ok := out["playbackKeyframes"]; ok {
		t.Fatalf("expected playbackKeyframes to be removed: %#v", out)
	}
	if got := out["outcome"]; got != "won" {
		t.Fatalf("outcome=%v, want won", got)
	}
	if _, ok := in["playbackKeyframes"]; !ok {
		t.Fatalf("input map should not be mutated")
	}
}
