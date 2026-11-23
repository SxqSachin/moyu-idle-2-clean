package httpapi

import (
	"strings"
	"time"
)

type battleTurnProfile struct {
	Turns        int64
	Errors       int64
	Terminal     int64
	Advanced     int64
	TotalMs      int64
	PrecheckMs   int64
	PreActionMs  int64
	ExecuteMs    int64
	PostActionMs int64
	FinalizeMs   int64
	MaxTurnMs    int64
}

type battleTurnProfileSample struct {
	PrecheckMs   int64
	PreActionMs  int64
	ExecuteMs    int64
	PostActionMs int64
	FinalizeMs   int64
}

func (sample battleTurnProfileSample) totalMs() int64 {
	return sample.PrecheckMs + sample.PreActionMs + sample.ExecuteMs + sample.PostActionMs + sample.FinalizeMs
}

func battleDurationMs(startedAt time.Time) int64 {
	return maxInt64(0, time.Since(startedAt).Milliseconds())
}

func (state *battleRuntimeState) commitTurnProfileSample(sample battleTurnProfileSample, status string, advanced bool, err error) {
	if state == nil {
		return
	}
	profile := &state.turnProfile
	profile.Turns++
	if err != nil {
		profile.Errors++
	}
	if status != "" {
		profile.Terminal++
	}
	if advanced {
		profile.Advanced++
	}
	totalMs := sample.totalMs()
	profile.TotalMs += totalMs
	profile.PrecheckMs += sample.PrecheckMs
	profile.PreActionMs += sample.PreActionMs
	profile.ExecuteMs += sample.ExecuteMs
	profile.PostActionMs += sample.PostActionMs
	profile.FinalizeMs += sample.FinalizeMs
	if totalMs > profile.MaxTurnMs {
		profile.MaxTurnMs = totalMs
	}
}

func (profile battleTurnProfile) toMap() map[string]any {
	turns := maxInt64(1, profile.Turns)
	return map[string]any{
		"turns":           profile.Turns,
		"errors":          profile.Errors,
		"terminalTurns":   profile.Terminal,
		"advancedTurns":   profile.Advanced,
		"totalMs":         profile.TotalMs,
		"maxTurnMs":       profile.MaxTurnMs,
		"precheckMs":      profile.PrecheckMs,
		"preActionMs":     profile.PreActionMs,
		"executeMs":       profile.ExecuteMs,
		"postActionMs":    profile.PostActionMs,
		"finalizeMs":      profile.FinalizeMs,
		"avgTurnMs":       roundTo4(float64(profile.TotalMs) / float64(turns)),
		"avgExecuteMs":    roundTo4(float64(profile.ExecuteMs) / float64(turns)),
		"avgPostActionMs": roundTo4(float64(profile.PostActionMs) / float64(turns)),
	}
}

func battleCheckpointTurnProfileMetric(checkpoint map[string]any, key string) int64 {
	if len(checkpoint) == 0 || strings.TrimSpace(key) == "" {
		return 0
	}
	raw, ok := checkpoint["turnProfileMs"]
	if !ok {
		return 0
	}
	profile := normalizeAnyMap(raw)
	if len(profile) == 0 {
		return 0
	}
	return toInt64Loose(profile[key], 0)
}
