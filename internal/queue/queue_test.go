package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"anvil/internal/config"
	"anvil/internal/store"
)

func newQueue(t *testing.T) (*Queue, *store.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, config.DatabaseConfig{Driver: "sqlite"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Migrate(ctx, false); err != nil {
		t.Fatal(err)
	}
	return New(db, time.Minute), db
}

func TestRetryThenSucceed(t *testing.T) {
	ctx := context.Background()
	q, db := newQueue(t)
	clock := time.Unix(1000, 0)
	q.now = func() time.Time { return clock }
	calls := 0
	q.Handle("k", func(ctx context.Context, task Task) error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return nil
	})
	q.Enqueue(ctx, "k", map[string]int{"n": 1})
	if ran, err := q.RunOne(ctx); !ran || err != nil {
		t.Fatal(ran, err)
	}
	if ran, _ := q.RunOne(ctx); ran {
		t.Fatal("retried before its backoff")
	}
	clock = clock.Add(time.Hour)
	if ran, err := q.RunOne(ctx); !ran || err != nil {
		t.Fatal(ran, err)
	}
	if n, _ := db.CountTasks(ctx, "done"); n != 1 || calls != 2 {
		t.Fatalf("done=%d calls=%d", n, calls)
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	q, db := newQueue(t)
	clock := time.Unix(1000, 0)
	q.now = func() time.Time { return clock }
	q.Handle("k", func(context.Context, Task) error { panic("bug") })
	q.Enqueue(ctx, "k", nil)
	for i := 0; i < MaxAttempts+2; i++ {
		q.RunOne(ctx)
		clock = clock.Add(time.Hour)
	}
	if n, _ := db.CountTasks(ctx, "failed"); n != 1 {
		t.Fatalf("failed = %d", n)
	}
}

func TestWorkersRunTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q, _ := newQueue(t)
	done := make(chan int64, 10)
	q.Handle("k", func(_ context.Context, task Task) error { done <- task.ID; return nil })
	wait := q.Start(ctx, 2)
	for i := 0; i < 5; i++ {
		q.Enqueue(ctx, "k", i)
	}
	for i := 0; i < 5; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("task not run")
		}
	}
	cancel()
	wait()
}
