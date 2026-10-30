package events

import (
	"context"
	"testing"
	"time"

	"anvil/internal/config"
	"anvil/internal/store"
)

func newBus(t *testing.T) *Bus {
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
	return NewBus(db)
}

func TestPublishSubscribe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newBus(t)
	sub, err := b.Subscribe(ctx, Filter{TypePrefix: "ci."}, 0)
	if err != nil {
		t.Fatal(err)
	}
	b.Publish(ctx, Event{Type: "git.push"})
	b.Publish(ctx, Event{Type: "ci.run.created", Payload: map[string]int{"run": 1}})
	select {
	case ev := <-sub.C:
		if ev.Type != "ci.run.created" || ev.ID == 0 {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event delivered")
	}
}

func TestReplayAfterID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newBus(t)
	first := Event{Type: "a"}
	b.Publish(ctx, first)
	for i := 0; i < 3; i++ {
		b.Publish(ctx, Event{Type: "b"})
	}
	sub, _ := b.Subscribe(ctx, Filter{}, 1)
	for i := 0; i < 3; i++ {
		if ev := <-sub.C; ev.Type != "b" {
			t.Fatalf("replayed %+v", ev)
		}
	}
}

func TestSlowSubscriberGetsGap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newBus(t)
	sub, _ := b.Subscribe(ctx, Filter{}, 0)
	for i := 0; i < 300; i++ { // more than the channel buffer
		if err := b.Publish(ctx, Event{Type: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 256; i++ {
		<-sub.C
	}
	b.Publish(ctx, Event{Type: "x"})
	if ev := <-sub.C; !ev.Gap {
		t.Fatalf("expected a gap marker, got %+v", ev)
	}
}
