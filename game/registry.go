package game

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// OnChangeFunc is called when the game list changes. games is shared and
// must not be modified.
type OnChangeFunc func(games []Game)

// Registry maintains a thread-safe collection of discovered games.
//
// Writers publish an immutable snapshot, sorted by Key, that readers load
// without locking. onChange runs on Run's goroutine, never under the lock,
// so a slow subscriber cannot hold up writers or readers.
type Registry struct {
	mu       sync.Mutex // serializes writers
	games    map[string]Game
	snapshot atomic.Pointer[[]Game]
	changed  chan struct{}
	onChange OnChangeFunc
}

// NewRegistry creates a new game registry.
func NewRegistry(onChange OnChangeFunc) *Registry {
	r := &Registry{
		games:    make(map[string]Game),
		changed:  make(chan struct{}, 1),
		onChange: onChange,
	}
	r.snapshot.Store(&[]Game{})

	return r
}

// Run expires games unseen for ttl and calls onChange with the latest
// snapshot after changes, until ctx is done. Changes made while onChange
// runs coalesce into one call.
func (r *Registry) Run(ctx context.Context, ttl time.Duration) error {
	// Separate goroutine: a stalled subscriber must not keep dead games
	// advertised.
	go r.expireLoop(ctx, ttl)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.changed:
			if r.onChange != nil {
				r.onChange(r.Games())
			}
		}
	}
}

// Add adds or updates a game in the registry.
// Returns true if the game was newly added.
func (r *Registry) Add(game Game) bool {
	key := game.Key()

	r.mu.Lock()
	old, exists := r.games[key]
	r.games[key] = upsert(old, game, time.Now())
	total := r.publish()
	r.mu.Unlock()

	if !exists {
		slog.Debug(
			"adding new game to registry",
			"key", key,
			"name", game.Info.GameName,
			"hostCounter", game.Info.HostCounter,
			"peerIP", game.PeerIP,
			"source", game.Source,
			"totalGames", total,
		)
	}

	r.notify()

	return !exists
}

// upsert merges a fresh observation into the stored game. Observations come
// straight off the wire and carry no history, so FirstSeen comes from old;
// a zero old means a first sighting.
func upsert(old, obs Game, now time.Time) Game {
	obs.FirstSeen = old.FirstSeen
	if obs.FirstSeen.IsZero() {
		obs.FirstSeen = now
	}

	obs.LastSeen = now

	return obs
}

// Games returns all games sorted by Key. The slice is shared and must not
// be modified.
func (r *Registry) Games() []Game {
	return *r.snapshot.Load()
}

// LocalGames returns games hosted locally.
func (r *Registry) LocalGames() []Game {
	var local []Game

	for _, g := range r.Games() {
		if g.Source == SourceLocal {
			local = append(local, g)
		}
	}

	return local
}

// FindByHostCounter finds a remote game by its HostCounter.
// Returns nil if not found.
func (r *Registry) FindByHostCounter(hostCounter uint32) *Game {
	for _, g := range r.Games() {
		if g.Source == SourceRemote && g.Info.HostCounter == hostCounter {
			return &g
		}
	}

	return nil
}

// Expire removes games last seen before cutoff.
// Returns the number of games removed.
func (r *Registry) Expire(cutoff time.Time) int {
	r.mu.Lock()
	before := len(r.games)
	maps.DeleteFunc(r.games, func(_ string, g Game) bool { return g.LastSeen.Before(cutoff) })
	removed := before - len(r.games)

	if removed > 0 {
		r.publish()
	}
	r.mu.Unlock()

	if removed > 0 {
		slog.Debug("expired games", "removed", removed)
		r.notify()
	}

	return removed
}

// expireChecksPerTTL bounds how long a dead game lingers to
// ttl + ttl/expireChecksPerTTL after its last sighting.
const expireChecksPerTTL = 2

// expireLoop expires games unseen for ttl until ctx is done.
func (r *Registry) expireLoop(ctx context.Context, ttl time.Duration) {
	ticker := time.NewTicker(ttl / expireChecksPerTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			r.Expire(now.Add(-ttl))
		}
	}
}

// publish stores a fresh snapshot of r.games and returns its length.
// Must be called with r.mu held.
func (r *Registry) publish() int {
	games := slices.SortedFunc(maps.Values(r.games), func(a, b Game) int {
		return strings.Compare(a.Key(), b.Key())
	})
	r.snapshot.Store(&games)

	return len(games)
}

// notify wakes Run without blocking. A pending wake-up already covers this
// change, since Run reads the snapshot only when it wakes.
func (r *Registry) notify() {
	select {
	case r.changed <- struct{}{}:
	default:
	}
}
