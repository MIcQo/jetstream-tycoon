// Package sim contains pure, deterministic game simulation logic (ADR-0002, ADR-0006).
package sim

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
)

// State holds the minimal deterministic simulation state stub for M0 gates.
type State struct {
	Version   int
	Seed      uint64
	Tick      int64
	CashCents int64
}

// New creates an initial deterministic simulation state.
func New(seed uint64) *State {
	return &State{
		Version:   1,
		Seed:      seed,
		Tick:      0,
		CashCents: 100_000_00,
	}
}

// Advance advances the simulation by ticks game hours deterministically.
func (s *State) Advance(ticks int) {
	for i := 0; i < ticks; i++ {
		s.Tick++
		factor := detmath.Sqrt(float64(s.Tick) + 1.0)
		delta := detmath.RoundCents(float64(factor * 100.0))
		s.CashCents += delta
	}
}

// Hash computes a deterministic SHA-256 hex digest of the simulation state.
func (s *State) Hash() string {
	var buf [32]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(s.Version))
	binary.LittleEndian.PutUint64(buf[8:16], s.Seed)
	binary.LittleEndian.PutUint64(buf[16:24], uint64(s.Tick))
	binary.LittleEndian.PutUint64(buf[24:32], uint64(s.CashCents))
	sum := sha256.Sum256(buf[:])
	return hex.EncodeToString(sum[:])
}
