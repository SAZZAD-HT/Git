package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"anvil/internal/store"
)

// Event is one thing that happened. Payload is marshalled to JSON when stored.
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	RepoID    int64     `json:"repo_id,omitempty"`
	ActorKind string    `json:"actor_kind"` // user, runner or system
	ActorID   int64     `json:"actor_id,omitempty"`
	Payload   any       `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
	// Gap is true on a marker event telling a subscriber it missed events before this point.
	Gap bool `json:"gap,omitempty"`
}

// Filter selects events for a subscription. Zero fields match everything.
type Filter struct {
	TypePrefix string // e.g. "ci." for every CI event
	RepoID     int64
}

func (f Filter) match(ev Event) bool {
	return strings.HasPrefix(ev.Type, f.TypePrefix) && (f.RepoID == 0 || f.RepoID == ev.RepoID)
}

// Subscription receives matching events on C until its context ends; C is then closed.
type Subscription struct {
	C      <-chan Event
	ch     chan Event
	filter Filter
	gapped bool
}

// Bus stores events and fans them out to subscribers. Safe for concurrent use.
type Bus struct {
	db   *store.DB
	mu   sync.Mutex
	subs map[*Subscription]struct{}
	now  func() time.Time
}

// NewBus returns a Bus that persists events in db.
func NewBus(db *store.DB) *Bus {
	return &Bus{db: db, subs: map[*Subscription]struct{}{}, now: time.Now}
}

// Publish stores ev in the events table and delivers it to matching in-process subscribers.
// Delivery to a slow subscriber is dropped rather than blocking (the subscriber sees a gap marker).
// It sets ev.ID and ev.CreatedAt, and returns an error only if the event could not be stored.
func (b *Bus) Publish(ctx context.Context, ev Event) error {
	if ev.ActorKind == "" {
		ev.ActorKind = "system"
	}
	ev.CreatedAt = b.now().UTC()
	payload, err := json.Marshal(ev.Payload)
	if err != nil {
		return fmt.Errorf("event %s: marshal payload: %w", ev.Type, err)
	}
	err = b.db.Tx(ctx, func(tx *store.Tx) error {
		ev.ID, err = tx.InsertEvent(ctx, store.EventRow{
			Type: ev.Type, RepoID: ev.RepoID, ActorKind: ev.ActorKind, ActorID: ev.ActorID,
			Payload: string(payload), CreatedAt: ev.CreatedAt.UnixMilli(),
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("event %s: %w", ev.Type, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs {
		if !s.filter.match(ev) {
			continue
		}
		if s.gapped {
			select {
			case s.ch <- Event{ID: ev.ID - 1, Type: "gap", Gap: true}:
				s.gapped = false
			default:
				continue
			}
		}
		select {
		case s.ch <- ev:
		default:
			s.gapped = true
		}
	}
	return nil
}

// Subscribe returns a subscription receiving events that match f, starting after afterID.
// Stored events newer than afterID are replayed first (pass 0 to receive only new events
// from now on, or the last id seen to resume). The subscription ends when ctx is done.
func (b *Bus) Subscribe(ctx context.Context, f Filter, afterID int64) (*Subscription, error) {
	ch := make(chan Event, 256)
	s := &Subscription{C: ch, ch: ch, filter: f}
	b.mu.Lock() // hold the lock across the replay so no event is published in between
	if afterID > 0 {
		rows, err := b.db.EventsAfter(ctx, afterID, cap(ch))
		if err != nil {
			b.mu.Unlock()
			return nil, err
		}
		for _, r := range rows {
			ev := Event{ID: r.ID, Type: r.Type, RepoID: r.RepoID, ActorKind: r.ActorKind, ActorID: r.ActorID,
				Payload: json.RawMessage(r.Payload), CreatedAt: time.UnixMilli(r.CreatedAt).UTC()}
			if f.match(ev) {
				ch <- ev
			}
		}
	}
	b.subs[s] = struct{}{}
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subs, s)
		close(ch)
		b.mu.Unlock()
	}()
	return s, nil
}
