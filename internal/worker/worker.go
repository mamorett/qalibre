package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Task Status Constants (cps/services/worker.py:36-41)
const (
	StatWaiting       = 0
	StatFailed        = 1
	StatStarted       = 2
	StatFinishSuccess = 3
	StatEnded         = 4
	StatCancelled     = 5
)

// Task interface
type Task interface {
	Name() string
	IsCancellable() bool
	Run(ctx context.Context, db interface{}) error
}

type TaskInfo struct {
	ID            string    `json:"task_id"`
	User          string    `json:"user"`
	Task          Task      `json:"-"`
	Name          string    `json:"name"`
	Message       string    `json:"message"`
	Progress      float64   `json:"progress"`
	Stat          int       `json:"stat"`
	Error         string    `json:"error"`
	StartTime     time.Time `json:"start_time"`
	Runtime       string    `json:"runtime"`
	IsCancellable bool      `json:"is_cancellable"`
}

type Worker struct {
	mu     sync.RWMutex
	tasks  []*TaskInfo
	queue  chan *TaskInfo
	db     interface{}
	cancel map[string]context.CancelFunc
}

var (
	instance *Worker
	once     sync.Once
)

// GetInstance returns the singleton Worker thread-safe instance
func GetInstance(db interface{}) *Worker {
	once.Do(func() {
		instance = &Worker{
			tasks:  make([]*TaskInfo, 0),
			queue:  make(chan *TaskInfo, 100),
			db:     db,
			cancel: make(map[string]context.CancelFunc),
		}
		go instance.start()
	})
	return instance
}

func (w *Worker) start() {
	slog.Info("background worker: starting serial execution queue")
	for info := range w.queue {
		w.mu.Lock()
		if info.Stat == StatCancelled {
			w.mu.Unlock()
			continue
		}
		info.Stat = StatStarted
		info.StartTime = time.Now()
		ctx, cancelFunc := context.WithCancel(context.Background())
		w.cancel[info.ID] = cancelFunc
		w.mu.Unlock()

		slog.Info("background worker: running task", "name", info.Name, "user", info.User)

		err := info.Task.Run(ctx, w.db)

		w.mu.Lock()
		delete(w.cancel, info.ID)
		if err != nil {
			info.Stat = StatFailed
			info.Error = err.Error()
			info.Progress = 1.0
			slog.Error("background worker: task failed", "name", info.Name, "err", err)
		} else {
			if info.Stat != StatCancelled && info.Stat != StatEnded {
				info.Stat = StatFinishSuccess
				info.Progress = 1.0
				slog.Info("background worker: task finished successfully", "name", info.Name)
			}
		}
		w.mu.Unlock()
	}
}

// AddTask enqueues a new task
func (w *Worker) AddTask(username string, t Task) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := fmt.Sprintf("task-%d", time.Now().UnixNano())
	info := &TaskInfo{
		ID:            id,
		User:          username,
		Task:          t,
		Name:          t.Name(),
		Stat:          StatWaiting,
		IsCancellable: t.IsCancellable(),
	}

	// Keep at most 50 past/queued tasks
	if len(w.tasks) >= 50 {
		w.tasks = w.tasks[1:]
	}

	w.tasks = append(w.tasks, info)
	w.queue <- info
	return id
}

// CancelTask cancels a queued/running task by ID
func (w *Worker) CancelTask(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, info := range w.tasks {
		if info.ID == id {
			if info.Stat == StatWaiting {
				info.Stat = StatCancelled
				info.Progress = 1.0
			} else if info.Stat == StatStarted {
				if info.IsCancellable {
					if cancel, ok := w.cancel[id]; ok {
						cancel()
					}
					info.Stat = StatCancelled
					info.Progress = 1.0
				} else {
					info.Stat = StatEnded
				}
			}
			break
		}
	}
}

// GetTasks returns a copy of the task status slice filtered for the current user (if not admin)
func (w *Worker) GetTasks(username string, isAdmin bool) []map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	res := make([]map[string]interface{}, 0)
	for _, t := range w.tasks {
		if t.User == username || isAdmin {
			item := map[string]interface{}{
				"task_id":        t.ID,
				"user":           t.User,
				"taskMessage":    t.Name,
				"progress":       fmt.Sprintf("%d %%", int(t.Progress*100)),
				"stat":           t.Stat,
				"is_cancellable": t.IsCancellable,
				"error":          t.Error,
			}

			if t.Stat == StatStarted {
				item["status"] = "Started"
				dur := time.Since(t.StartTime)
				item["runtime"] = fmt.Sprintf("%02d:%02d:%02d", int(dur.Hours()), int(dur.Minutes())%60, int(dur.Seconds())%60)
				item["starttime"] = t.StartTime.Format("2006-01-02 15:04:05")
			} else if t.Stat == StatWaiting {
				item["status"] = "Waiting"
			} else if t.Stat == StatFinishSuccess {
				item["status"] = "Finished"
			} else if t.Stat == StatFailed {
				item["status"] = "Failed"
			} else if t.Stat == StatCancelled {
				item["status"] = "Cancelled"
			} else {
				item["status"] = "Ended"
			}

			res = append(res, item)
		}
	}
	return res
}

// Shutdown stops the worker queue
func (w *Worker) Shutdown() {
	close(w.queue)
}
