// Package state holds the serializable simulation state (TECH_DESIGN §4).
// M0 keeps only the core fields; the full multi-airline model arrives in M2.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// TicksPerDay is the number of ticks (game hours) in a game day.
const TicksPerDay = 24

// SchemaVersion is the current save schema version (bumped with save migrations).
const SchemaVersion = 1

// State is the complete game state. Maps are for lookup only; logic iterates
// sorted keys (ADR-0006).
type State struct {
	Version   int    `json:"version"`
	Seed      uint64 `json:"seed"`
	Tick      int64  `json:"tick"`
	Speed     int    `json:"speed"`
	CashCents int64  `json:"cashCents"` // placeholder balance until the airline model (M2)
}

// New returns the initial state for a seed.
func New(seed uint64) *State {
	return &State{
		Version:   SchemaVersion,
		Seed:      seed,
		Speed:     1,
		CashCents: 100_000_00,
	}
}

// Day returns the current game day (0-based).
func (s *State) Day() int64 { return s.Tick / TicksPerDay }

// Hash returns a SHA-256 hex digest of the canonical JSON encoding. encoding/json
// writes struct fields in declaration order and map keys sorted, so the digest is
// identical on every platform.
func (s *State) Hash() string {
	b, err := json.Marshal(s)
	if err != nil {
		panic("state: marshal for hash: " + err.Error()) // State contains only marshalable fields
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
