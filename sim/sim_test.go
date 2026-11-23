package sim_test

import (
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim"
)

const expectedGoldenHash = "8c94d155a43185c2bf23642b35a9588a72963e6559bf9daad7810b2f70720760"

func TestDeterministicStateHash(t *testing.T) {
	s1 := sim.New(42)
	s1.Advance(24)

	s2 := sim.New(42)
	s2.Advance(24)

	if s1.Hash() != s2.Hash() {
		t.Fatalf("non-deterministic hash: %s != %s", s1.Hash(), s2.Hash())
	}
	if s1.Hash() != expectedGoldenHash {
		t.Fatalf("golden hash mismatch: got %s, want %s", s1.Hash(), expectedGoldenHash)
	}
}
