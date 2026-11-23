package sim_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/command"
)

// expectedGoldenHash is the state hash after the scripted game below. Update it
// only for intentional behaviour changes, and say why in the PR (ADR-0006).
const expectedGoldenHash = "8fe4257585f92e735772ad9dcc4793c945cebbd32c8367a92a65a00c730ec2a1"

func scriptedGame(seed uint64) string {
	e := sim.NewGame(seed)
	e.Advance(24)
	e.Enqueue(command.SetSpeed{Speed: 4})
	e.Advance(24 * 6)
	return e.State.Hash()
}

func TestDeterministicStateHash(t *testing.T) {
	h1, h2 := scriptedGame(42), scriptedGame(42)
	if h1 != h2 {
		t.Fatalf("non-deterministic hash: %s != %s", h1, h2)
	}
	if h1 != expectedGoldenHash {
		t.Fatalf("golden hash mismatch: got %s, want %s", h1, expectedGoldenHash)
	}
	if scriptedGame(43) == h1 {
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

	f.Fuzz(func(t *testing.T, seed uint64, steps uint8) {
		ticks := int(steps)
		a := sim.NewGame(seed)
		a.Advance(ticks)

		b := sim.NewGame(seed)
		b.Advance(ticks)

		if a.State.Tick != int64(ticks) {
			t.Fatalf("Tick = %d, want %d", a.State.Tick, ticks)
		}
		if a.State.Hash() != b.State.Hash() {
			t.Fatalf("hash mismatch for seed=%d ticks=%d", seed, ticks)
		}
	})
}
