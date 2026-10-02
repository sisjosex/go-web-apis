package jobs

import (
	"log"
	"time"

	coreServices "josex/web/modules/core/services"

	"github.com/hibiken/asynq"
)

// Worker runs the registered handlers against the three queues.
type Worker struct {
	server *asynq.Server
}

// StartWorker starts processing in the background. asynq retries a Valkey that is down on its own,
// so a worker that boots before Valkey starts working once it is reachable.
func StartWorker(valkey coreServices.ValkeyService, registry *Registry, concurrency int) (*Worker, error) {
	server := asynq.NewServerFromRedisClient(valkey.Client(), asynq.Config{
		Concurrency: concurrency,
		Queues:      queueWeights,
		LogLevel:    asynq.WarnLevel,
		// An idle worker looks for work every 200 ms rather than asynq's 1 s: a trip transition reaches
		// a watching socket within a second (TRACK-025), for five cheap Valkey checks a second.
		TaskCheckInterval: 200 * time.Millisecond,
		// A delayed task (a driver's grouped day_changed, TRACK-030) is moved to its queue within
		// 500 ms of its time rather than asynq's 5 s.
		DelayedTaskCheckInterval: 500 * time.Millisecond,
	})
	if err := server.Start(registry.mux); err != nil {
		return nil, err
	}
	log.Printf("✅ Worker started (concurrency %d, queues critical:6 default:3 low:1)", concurrency)
	return &Worker{server: server}, nil
}

// Shutdown stops pulling tasks and waits for the ones in flight, up to asynq's shutdown timeout;
// an unfinished task goes back to its queue and runs again elsewhere.
func (w *Worker) Shutdown() {
	w.server.Shutdown()
}
