package core

import (
	"fmt"
	"time"
)

type TurnStepFunc func(now time.Time) (status string, waveWon bool, err error)

type CompileOrchestratorInput struct {
	Now             time.Time
	TurnInterval    time.Duration
	InterBattleGap  time.Duration
	MaxSteps        int
	MaxExecDuration time.Duration
	ProcessTurnStep TurnStepFunc
}

type CompileOrchestratorResult struct {
	Outcome          string
	EndTime          time.Time
	Steps            int
	TimedOut         bool
	StepLimitReached bool
}

func RunCompileOrchestration(input CompileOrchestratorInput) (CompileOrchestratorResult, error) {
	if input.ProcessTurnStep == nil {
		return CompileOrchestratorResult{}, fmt.Errorf("process turn step is required")
	}
	if input.MaxSteps <= 0 {
		input.MaxSteps = 1000
	}
	if input.MaxExecDuration <= 0 {
		input.MaxExecDuration = 2500 * time.Millisecond
	}
	if input.TurnInterval <= 0 {
		input.TurnInterval = time.Second
	}
	if input.InterBattleGap < 0 {
		input.InterBattleGap = 0
	}
	simNow := input.Now
	startedAt := time.Now()
	outcome := ""
	steps := 0
	for step := 0; step < input.MaxSteps; step++ {
		steps = step + 1
		status, waveWon, err := input.ProcessTurnStep(simNow)
		if err != nil {
			return CompileOrchestratorResult{}, err
		}
		if status != "" {
			outcome = status
			break
		}
		if time.Since(startedAt) >= input.MaxExecDuration {
			return CompileOrchestratorResult{
				Outcome:  "failed",
				EndTime:  simNow,
				Steps:    steps,
				TimedOut: true,
			}, nil
		}
		if waveWon && input.InterBattleGap > 0 {
			simNow = simNow.Add(input.InterBattleGap)
			continue
		}
		simNow = simNow.Add(input.TurnInterval)
	}
	if outcome == "" {
		return CompileOrchestratorResult{
			Outcome:          "failed",
			EndTime:          simNow,
			Steps:            steps,
			StepLimitReached: true,
		}, nil
	}
	return CompileOrchestratorResult{
		Outcome: outcome,
		EndTime: simNow,
		Steps:   steps,
	}, nil
}
