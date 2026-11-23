package battleruntime

import (
	"context"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/httpapi"
)

type Worker struct {
	runtime *httpapi.Server
}

func NewWorker(deps app.Dependencies) *Worker {
	return &Worker{runtime: httpapi.NewBattleWorker(deps)}
}

func (w *Worker) Close(ctx context.Context) error {
	if w == nil || w.runtime == nil {
		return nil
	}
	return w.runtime.ShutdownBattleWorker(ctx)
}
