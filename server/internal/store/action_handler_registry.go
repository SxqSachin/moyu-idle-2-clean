package store

import (
	"errors"
	"strings"
)

// actionHandlerForType routes a queue item to its business handler by action type.
// Queue core owns scheduling/orchestration only; business logic stays in handlers.
func (s *Store) actionHandlerForType(actionType string) QueueActionExecutor {
	normalized := strings.TrimSpace(actionType)
	if normalized == "" {
		return newGenericExecutor(s)
	}
	return s.executorForAction(normalized)
}

type preparedActionQueueStep struct {
	spec        ActionSpec
	executor    QueueActionExecutor
	args        any
	insertCount int
}

// Validate the whole command before opening a transaction. Client args never
// become a runtime payload; only the selected executor can freeze that payload.
func (s *Store) prepareActionQueueStep(specs map[string]ActionSpec, step ActionBatchStep) (preparedActionQueueStep, error) {
	if step.Count <= 0 {
		return preparedActionQueueStep{}, ErrInvalidActionCount
	}
	if strings.TrimSpace(step.ActionID) == "alchemy.inherit" {
		return preparedActionQueueStep{}, ErrAlchemyArgsInvalid
	}
	spec, ok := specs[step.ActionID]
	if !ok {
		return preparedActionQueueStep{}, errors.New("action spec not found")
	}
	executor := s.actionHandlerForType(step.ActionID)
	args, err := executor.ValidateAndNormalizeArgs(step.ArgsRaw)
	if err != nil {
		return preparedActionQueueStep{}, err
	}
	if single, ok := args.(queueSingleRunArgs); ok && single.RequiresSingleQueueRun() && step.Count != 1 {
		return preparedActionQueueStep{}, ErrInvalidActionCount
	}
	if binding, ok := args.(actionMaterialBindingArgs); ok && binding.HasMaterialInstanceBindings() && step.Count != 1 {
		return preparedActionQueueStep{}, ErrInvalidActionCount
	}
	insertCount, err := resolveQueueInsertCount(step.ActionID, step.Count, args)
	if err != nil {
		return preparedActionQueueStep{}, err
	}
	return preparedActionQueueStep{spec: spec, executor: executor, args: args, insertCount: insertCount}, nil
}
