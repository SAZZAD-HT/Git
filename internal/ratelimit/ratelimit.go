package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most Limit events per key in each Window. Safe for concurrent use.
type Limiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu        sync.Mutex
	counts    map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	start time.Time
	n     int
}

// New returns a Limiter allowing limit events per key per window.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, now: time.Now, counts: map[string]*bucket{}}
}

// Allow records one event for key and reports whether it is within the limit. When it is
// not, retryAfter says how long until the window resets.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	b := l.counts[key]
	if b == nil || now.Sub(b.start) >= l.window {
		b = &bucket{start: now}
		l.counts[key] = b
	}
	if b.n >= l.limit {
		return false, b.start.Add(l.window).Sub(now)
	}
	b.n++
	return true, 0
}

// Exceeded reports whether key has already used its limit, without recording an event.
// Used to reject sign-ins before checking the password once too many have failed.
func (l *Limiter) Exceeded(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.counts[key]
	if b == nil || now.Sub(b.start) >= l.window || b.n < l.limit {
		return false, 0
	}
	return true, b.start.Add(l.window).Sub(now)
}

// Reset forgets key, e.g. after a successful sign-in.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.counts, key)
	l.mu.Unlock()
}

// sweep drops expired buckets once per window so memory stays bounded. Caller holds mu.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for k, b := range l.counts {
		if now.Sub(b.start) >= l.window {
			delete(l.counts, k)
		}
	}
}
