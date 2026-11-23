package sim_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim"
)

const expectedGoldenHash = "8c94d155a43185c2bf23642b35a9588a72963e6559bf9daad7810b2f70720760"

func TestDeterministicStateHash(t *testing.T) {
	s1 := sim.New(42)
	if s1.Version != 1 || s1.Seed != 42 || s1.Tick != 0 || s1.CashCents != 100_000_00 {
		t.Fatalf("unexpected initial state: %+v", s1)
	}
	s1.Advance(24)
	if s1.Tick != 24 || s1.CashCents <= 100_000_00 {
		t.Fatalf("unexpected state after 24 ticks: %+v", s1)
	}

	s2 := sim.New(42)
	s2.Advance(24)

	if s1.Hash() != s2.Hash() {
		t.Fatalf("non-deterministic hash: %s != %s", s1.Hash(), s2.Hash())
	}
	if s1.Hash() != expectedGoldenHash {
		t.Fatalf("golden hash mismatch: got %s, want %s", s1.Hash(), expectedGoldenHash)
	}
}

func FuzzStateAdvance(f *testing.F) {
	f.Add(uint64(1), uint8(1))
	f.Add(uint64(42), uint8(24))
	f.Add(uint64(999999), uint8(100))

	f.Fuzz(func(t *testing.T, seed uint64, steps uint8) {
		ticks := int(steps)
		a := sim.New(seed)
		a.Advance(ticks)

		b := sim.New(seed)
		b.Advance(ticks)

		if a.Tick != int64(ticks) {
			t.Fatalf("Tick = %d, want %d", a.Tick, ticks)
		}
		if a.Hash() != b.Hash() {
			t.Fatalf("hash mismatch for seed=%d ticks=%d: %s != %s", seed, ticks, a.Hash(), b.Hash())
		}
	})
}
