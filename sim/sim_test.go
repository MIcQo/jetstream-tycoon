package sim_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/content/bundle"
)

// expectedGoldenHash is the state hash after the scripted game below. Update it
// only for intentional behaviour changes, and say why in the PR (ADR-0006).
const expectedGoldenHash = "80a36cbe4f1b8c6203e1f333d5a1b057b071aaa75f4a04b0a2da6706c3033ac6"

func realBundle(t testing.TB) *bundle.Bundle {
	t.Helper()
	b, err := bundle.Load()
	if err != nil {
		t.Fatal(err)
	}
	return &b
}

func scriptedGame(t testing.TB, seed uint64) string {
	t.Helper()
	e, err := sim.NewGame(sim.NewGameParams{Seed: seed, AirlineName: "Golden Air", HomeCountry: "GB", Difficulty: "normal"}, realBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	e.Advance(24)
	e.Enqueue(command.SetSpeed{Speed: 4})
	e.Advance(24 * 6)
	return e.State.Hash()
}

func TestDeterministicStateHash(t *testing.T) {
	h1, h2 := scriptedGame(t, 42), scriptedGame(t, 42)
	if h1 != h2 {
		t.Fatalf("non-deterministic hash: %s != %s", h1, h2)
	}
	if h1 != expectedGoldenHash {
		t.Fatalf("golden hash mismatch: got %s, want %s", h1, expectedGoldenHash)
	}
	if scriptedGame(t, 43) == h1 {
		t.Fatal("different seeds produced the same state")
	}
}

func TestSystemsOrder(t *testing.T) {
	systems := sim.Systems()
	if len(systems) != 1 || systems[0].Name() != "ledger" {
		t.Fatalf("systems = %v", systems)
	}
}

func FuzzStateAdvance(f *testing.F) {
	f.Add(uint64(1), uint8(1))
	f.Add(uint64(42), uint8(24))
	f.Add(uint64(999999), uint8(100))
	data := realBundle(f)

	f.Fuzz(func(t *testing.T, seed uint64, steps uint8) {
		p := sim.NewGameParams{Seed: seed, AirlineName: "Fuzz", HomeCountry: "FR", Difficulty: "easy"}
		a, err := sim.NewGame(p, data)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := sim.NewGame(p, data)
		a.Advance(int(steps))
		b.Advance(int(steps))
		if a.State.Tick != int64(steps) || a.State.Hash() != b.State.Hash() {
			t.Fatalf("seed=%d ticks=%d: tick %d, hashes differ: %v", seed, steps, a.State.Tick, a.State.Hash() != b.State.Hash())
		}
	})
}
