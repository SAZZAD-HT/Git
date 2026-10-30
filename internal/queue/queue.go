package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"anvil/internal/errs"
	"anvil/internal/logging"
	"anvil/internal/store"
)

// MaxAttempts is how many times a task runs before it is marked failed for good.
const MaxAttempts = 5

// Task is a claimed task handed to a handler.
type Task struct {
	ID       int64
	Kind     string
	Payload  json.RawMessage
	Attempts int // 1 on the first run
}

// Handler processes one task. Returning an error schedules a retry.
type Handler func(ctx context.Context, t Task) error

// Option changes how Enqueue stores a task.
type Option func(*options)

type options struct{ delay time.Duration }

// After delays the task's first run by d.
func After(d time.Duration) Option { return func(o *options) { o.delay = d } }

// Queue stores tasks and runs them on worker goroutines. Safe for concurrent use.
type Queue struct {
	db       *store.DB
	handlers map[string]Handler
	kinds    []string
	lease    time.Duration
	poll     time.Duration
	wake     chan struct{}
	started  bool
	mu       sync.Mutex
	now      func() time.Time
}

// New returns a Queue backed by db. Tasks are leased for lease (the longest a handler may run).
func New(db *store.DB, lease time.Duration) *Queue {
	return &Queue{db: db, handlers: map[string]Handler{}, lease: lease, poll: time.Second, wake: make(chan struct{}, 1), now: time.Now}
}

// Enqueue stores a durable task. It survives restarts and is retried with backoff on failure.
// payload is marshalled to JSON. It returns the task id. Workers are woken immediately.
func (q *Queue) Enqueue(ctx context.Context, kind string, payload any, opts ...Option) (int64, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("task %s: marshal payload: %w", kind, err)
	}
	now := q.now()
	var id int64
	err = q.db.Tx(ctx, func(tx *store.Tx) error {
		id, err = tx.InsertTask(ctx, kind, string(b), now.Add(o.delay).Unix(), now.Unix())
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("task %s: %w", kind, err)
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return id, nil
}

// Handle registers fn for tasks of kind. Must be called before Start; it panics otherwise,
// because a handler registered later would silently never run.
func (q *Queue) Handle(kind string, fn Handler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		panic("queue: Handle called after Start")
	}
	q.handlers[kind] = fn
	q.kinds = append(q.kinds, kind)
}

// Start runs n worker goroutines until ctx is cancelled, and returns a function that waits
// for them to finish their current task.
func (q *Queue) Start(ctx context.Context, n int) (wait func()) {
	q.mu.Lock()
	q.started = true
	q.mu.Unlock()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.work(ctx)
		}()
	}
	return wg.Wait
}

func (q *Queue) work(ctx context.Context) {
	for ctx.Err() == nil {
		ran, err := q.RunOne(ctx)
		if err != nil {
			logging.FromContext(ctx).Error("queue: claim failed", "err", err)
		}
		if ran {
			continue
		}
		select {
		case <-ctx.Done():
		case <-q.wake:
		case <-time.After(q.poll):
		}
	}
}

// RunOne claims and runs at most one ready task. It reports whether a task ran.
// Exposed for tests and for `anvil` subcommands that drain the queue synchronously.
func (q *Queue) RunOne(ctx context.Context) (bool, error) {
	now := q.now()
	var row *store.TaskRow
	err := q.db.Tx(ctx, func(tx *store.Tx) error {
		var err error
		row, err = tx.ClaimTask(ctx, q.kinds, now.Unix(), now.Add(q.lease).Unix())
		return err
	})
	if errors.Is(err, errs.NotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	task := Task{ID: row.ID, Kind: row.Kind, Payload: json.RawMessage(row.Payload), Attempts: row.Attempts}
	runErr := q.call(ctx, task)
	// Record the outcome with a fresh context: a shutdown must not lose a finished task's result.
	saveCtx := context.WithoutCancel(ctx)
	return true, q.db.Tx(saveCtx, func(tx *store.Tx) error {
		if runErr == nil {
			return tx.CompleteTask(saveCtx, task.ID)
		}
		logging.FromContext(ctx).Warn("queue: task failed", "task_id", task.ID, "kind", task.Kind, "attempt", task.Attempts, "err", runErr)
		backoff := time.Duration(1<<min(task.Attempts, 10)) * time.Second
		return tx.RetryTask(saveCtx, task.ID, runErr.Error(), q.now().Add(backoff).Unix(), task.Attempts >= MaxAttempts)
	})
}

// call runs the handler, turning a panic into an error so one bad task cannot kill a worker.
func (q *Queue) call(ctx context.Context, t Task) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, q.lease)
	defer cancel()
	return q.handlers[t.Kind](ctx, t)
}
