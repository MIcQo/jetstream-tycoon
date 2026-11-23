package state_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

func TestNew(t *testing.T) {
	s := state.New(42)
	if s.Version != state.SchemaVersion || s.Seed != 42 || s.Tick != 0 || s.Speed != 1 || s.CashCents != 100_000_00 {
		t.Fatalf("New(42) = %+v", s)
	}
}

func TestDay(t *testing.T) {
	cases := map[int64]int64{0: 0, 23: 0, 24: 1, 47: 1, 48: 2, 240: 10}
	for tick, want := range cases {
		s := state.New(1)
		s.Tick = tick
		if got := s.Day(); got != want {
			t.Errorf("Day() at tick %d = %d, want %d", tick, got, want)
		}
	}
}

func TestHash(t *testing.T) {
	a, b := state.New(1), state.New(1)
	if a.Hash() != b.Hash() {
		t.Fatal("equal states hash differently")
	}
	if len(a.Hash()) != 64 {
		t.Fatalf("hash %q is not 64 hex chars", a.Hash())
	}
	b.CashCents++
	if a.Hash() == b.Hash() {
		t.Fatal("different states hash equally")
	}
}
