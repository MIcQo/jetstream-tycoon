package state_test

import (
	"reflect"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

func TestNew(t *testing.T) {
	s := state.New(42)
	if s.Version != state.SchemaVersion || s.Seed != 42 || s.Tick != 0 || s.Speed != 1 || s.Difficulty != "normal" || s.NextID != 0 {
		t.Fatalf("New(42) = %+v", s)
	}
	// Every map is initialised, so systems can write without nil checks.
	if s.Airlines == nil || s.Countries == nil || s.Airports == nil || s.LandingRights == nil || s.Planes == nil ||
		s.Routes == nil || s.Markets == nil || s.Loans == nil || s.Offers == nil || s.Missions == nil ||
		s.Pending == nil || s.Modifiers == nil || s.Milestones == nil || s.ConfigSnapshot == nil {
		t.Fatal("a map is nil")
	}
	if s.Player() != nil {
		t.Error("an empty world has no player")
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

func TestNewIDIsSequential(t *testing.T) {
	s := state.New(1)
	if a, b, c := s.NewID("pl"), s.NewID("rt"), s.NewID("pl"); a != "pl1" || b != "rt2" || c != "pl3" || s.NextID != 3 {
		t.Fatalf("ids = %s %s %s, next %d", a, b, c, s.NextID)
	}
}

func TestPlayer(t *testing.T) {
	s := state.New(1)
	s.Airlines[state.PlayerID] = &state.Airline{ID: state.PlayerID, Name: "Me"}
	s.Airlines["A1"] = &state.Airline{ID: "A1"}
	if p := s.Player(); p == nil || p.Name != "Me" {
		t.Fatalf("Player() = %+v", p)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]int{"b": 1, "a": 2, "c": 3}
	if got := state.SortedKeys(m); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("SortedKeys = %v", got)
	}
	ids := map[state.ID]bool{"pl10": true, "pl2": true}
	if got := state.SortedKeys(ids); !reflect.DeepEqual(got, []state.ID{"pl10", "pl2"}) { // lexical order
		t.Errorf("SortedKeys(IDs) = %v", got)
	}
	if got := state.SortedKeys(map[int]bool{}); len(got) != 0 {
		t.Errorf("empty = %v", got)
	}
}

func TestKeys(t *testing.T) {
	if got := state.LandingRightKey("A1", "LHR"); got != "A1:LHR" {
		t.Errorf("LandingRightKey = %q", got)
	}
	if a, b := state.MarketKey("LHR", "CDG"), state.MarketKey("CDG", "LHR"); a != "CDG-LHR" || b != a {
		t.Errorf("MarketKey = %q %q", a, b)
	}
	if got := state.MarketKey("AAA", "AAA"); got != "AAA-AAA" {
		t.Errorf("MarketKey(same) = %q", got)
	}
}

func TestHash(t *testing.T) {
	a, b := state.New(1), state.New(1)
	if a.Hash() != b.Hash() || len(a.Hash()) != 64 {
		t.Fatal("equal states hash differently, or the hash is malformed")
	}
	b.Airlines["P"] = &state.Airline{ID: "P", CashCents: 1}
	if a.Hash() == b.Hash() {
		t.Fatal("different states hash equally")
	}
	// Map insertion order does not matter (encoding/json sorts keys).
	c, d := state.New(1), state.New(1)
	c.Planes["x"], c.Planes["y"] = &state.Plane{ID: "x"}, &state.Plane{ID: "y"}
	d.Planes["y"], d.Planes["x"] = &state.Plane{ID: "y"}, &state.Plane{ID: "x"}
	if c.Hash() != d.Hash() {
		t.Error("hash depends on map insertion order")
	}
}
