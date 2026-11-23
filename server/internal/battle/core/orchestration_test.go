package core

import (
	"testing"
	"time"
)

func TestRunCompileOrchestrationMarksFailedWhenExceedMaxSteps(t *testing.T) {
	result, err := RunCompileOrchestration(CompileOrchestratorInput{
		Now:             time.Unix(0, 0),
		TurnInterval:    time.Second,
		MaxSteps:        3,
		MaxExecDuration: time.Second,
		ProcessTurnStep: func(now time.Time) (string, bool, error) {
			return "", false, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != "failed" {
		t.Fatalf("outcome=%q, want failed", result.Outcome)
	}
	if !result.StepLimitReached {
		t.Fatalf("expected step limit reached")
	}
	if result.TimedOut {
		t.Fatalf("did not expect timeout")
	}
}

func TestRunCompileOrchestrationReturnsEndedWhenProcessEnds(t *testing.T) {
	steps := 0
	result, err := RunCompileOrchestration(CompileOrchestratorInput{
		Now:             time.Unix(0, 0),
		TurnInterval:    time.Second,
		MaxSteps:        5,
		MaxExecDuration: time.Second,
		ProcessTurnStep: func(now time.Time) (string, bool, error) {
			steps++
			if steps == 2 {
				return "ended", false, nil
			}
			return "", false, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != "ended" {
		t.Fatalf("outcome=%q, want ended", result.Outcome)
	}
	if result.Steps != 2 {
		t.Fatalf("steps=%d, want 2", result.Steps)
	}
}

func TestRunCompileOrchestrationMarksFailedWhenTimedOut(t *testing.T) {
	result, err := RunCompileOrchestration(CompileOrchestratorInput{
		Now:             time.Unix(0, 0),
		TurnInterval:    time.Second,
		MaxSteps:        1000,
		MaxExecDuration: 2 * time.Millisecond,
		ProcessTurnStep: func(now time.Time) (string, bool, error) {
			time.Sleep(3 * time.Millisecond)
			return "", false, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != "failed" {
		t.Fatalf("outcome=%q, want failed", result.Outcome)
	}
	if !result.TimedOut {
		t.Fatalf("expected timeout")
	}
	if result.StepLimitReached {
		t.Fatalf("did not expect step limit reached")
	}
}
