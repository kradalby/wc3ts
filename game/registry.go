package game

import (
	"log/slog"
	"sync"
	"time"
)

// OnChangeFunc is called when the game list changes.
type OnChangeFunc func(games []Game)

// Registry maintains a thread-safe collection of discovered games.
type Registry struct {
	games    map[string]*Game
	onChange OnChangeFunc
	mu       sync.RWMutex
}

// NewRegistry creates a new game registry.
func NewRegistry(onChange OnChangeFunc) *Registry {
	return &Registry{
		games:    make(map[string]*Game),
		onChange: onChange,
	}
}

// Add adds or updates a game in the registry.
// Returns true if the game was newly added.
func (r *Registry) Add(game Game) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := game.Key()
	_, exists := r.games[key]

	if !exists {
		game.FirstSeen = time.Now()
		slog.Debug("adding new game to registry",
			"key", key,
			"name", game.Info.GameName,
			"hostCounter", game.Info.HostCounter,
			"peerIP", game.PeerIP,
			"source", game.Source,
			"totalGames", len(r.games)+1,
		)
	}

	game.LastSeen = time.Now()
	r.games[key] = &game

	if r.onChange != nil {
		r.onChange(r.snapshot())
	}

	return !exists
}

// Remove removes a game from the registry.
// Returns true if the game existed.
func (r *Registry) Remove(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.games[key]
	if !exists {
		return false
	}

	delete(r.games, key)

	if r.onChange != nil {
		r.onChange(r.snapshot())
	}

	return true
}

// Games returns a copy of all games.
func (r *Registry) Games() []Game {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.snapshot()
}

// LocalGames returns games hosted locally.
func (r *Registry) LocalGames() []Game {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Game, 0)

	for _, g := range r.games {
		if g.Source == SourceLocal {
			result = append(result, *g)
		}
	}

	return result
}

// RemoteGames returns games from remote peers.
func (r *Registry) RemoteGames() []Game {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Game, 0)

	for _, g := range r.games {
		if g.Source == SourceRemote {
			result = append(result, *g)
		}
	}

	return result
}

// FindByHostCounter finds a remote game by its HostCounter.
// Returns nil if not found.
func (r *Registry) FindByHostCounter(hostCounter uint32) *Game {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, g := range r.games {
		if g.Source == SourceRemote && g.Info.HostCounter == hostCounter {
			gameCopy := *g

			return &gameCopy
		}
	}

	return nil
}

// Expire removes games that haven't been seen recently.
// Returns the number of games removed.
func (r *Registry) Expire(timeout time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0

	for key, game := range r.games {
		if game.IsStale(timeout) {
			delete(r.games, key)

			removed++
		}
	}

	if removed > 0 && r.onChange != nil {
		r.onChange(r.snapshot())
	}

	return removed
}

// snapshot returns a copy of all games.
// Must be called with at least a read lock held.
func (r *Registry) snapshot() []Game {
	result := make([]Game, 0, len(r.games))

	for _, g := range r.games {
		result = append(result, *g)
	}

	return result
}
