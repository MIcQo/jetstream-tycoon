package effects_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/effects"
	"github.com/MIcQo/jetstream-tycoon/sim/engine"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

func world() *state.State {
	s := state.New(1)
	s.Tick = 100
	return s
}

func add(t *testing.T, s *state.State, m state.Modifier) {
	t.Helper()
	if err := effects.Add(s, m); err != nil {
		t.Fatal(err)
	}
}

var lhr = effects.Target{Airline: "P", Country: "GB", Airport: "LHR", Route: "rt1", Plane: "pl1"}

func TestApplyWithoutModifiersReturnsClampedBase(t *testing.T) {
	s := world()
	if got := effects.Apply(s, effects.StatDemand, 1.5, lhr); got != 1.5 {
		t.Errorf("no modifiers = %v", got)
	}
	if got := effects.Apply(s, effects.StatDemand, 10, lhr); got != 3 {
		t.Errorf("base above the stat max = %v, want clamped to 3", got)
	}
	if got := effects.Apply(s, effects.StatDemand, 0, lhr); got != 0.2 {
		t.Errorf("base below the stat min = %v, want clamped to 0.2", got)
	}
	if got := effects.Apply(s, "unregistered", 99, lhr); got != 99 {
		t.Errorf("unknown stats are not clamped: %v", got)
	}
}

func TestStackingOrder(t *testing.T) {
	s := world()
	// (base + Σadd) × Πmul: (1 + 0.5 + 0.25) × 1.2 × 0.5 = 1.05
	add(t, s, state.Modifier{Source: "a", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpAdd, Value: 0.5})
	add(t, s, state.Modifier{Source: "b", Scope: effects.ScopeAirport, Target: "LHR", Stat: effects.StatDemand, Op: effects.OpAdd, Value: 0.25})
	add(t, s, state.Modifier{Source: "c", Scope: effects.ScopeCountry, Target: "GB", Stat: effects.StatDemand, Op: effects.OpMul, Value: 1.2})
	add(t, s, state.Modifier{Source: "d", Scope: effects.ScopeRoute, Target: "rt1", Stat: effects.StatDemand, Op: effects.OpMul, Value: 0.5})
	if got := effects.Apply(s, effects.StatDemand, 1, lhr); got != 1.05 {
		t.Errorf("stacked = %v, want 1.05", got)
	}
	// A clamp applies after stacking.
	add(t, s, state.Modifier{Source: "e", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 10})
	if got := effects.Apply(s, effects.StatDemand, 1, lhr); got != 3 {
		t.Errorf("clamped = %v, want 3", got)
	}
	// A set override replaces the clamped value.
	add(t, s, state.Modifier{Source: "f", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpSet, Value: 0.1})
	if got := effects.Apply(s, effects.StatDemand, 1, lhr); got != 0.1 {
		t.Errorf("set = %v, want 0.1 (not clamped)", got)
	}
}

func TestLastSetInSortedOrderWins(t *testing.T) {
	s := world()
	// Scope rank decides first: plane (rank 5) sorts after global (rank 0).
	add(t, s, state.Modifier{Source: "z", Scope: effects.ScopePlane, Target: "pl1", Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 0})
	add(t, s, state.Modifier{Source: "a", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 1})
	if got := effects.Apply(s, effects.StatAirportClosed, 0, lhr); got != 0 {
		t.Errorf("scope order: %v, want the plane-scoped set (0)", got)
	}
	// Same scope: higher priority sorts later and wins.
	s = world()
	add(t, s, state.Modifier{Source: "hi", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 1, Priority: 5})
	add(t, s, state.Modifier{Source: "lo", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 0, Priority: 1})
	if got := effects.Apply(s, effects.StatAirportClosed, 0, lhr); got != 1 {
		t.Errorf("priority order: %v, want 1", got)
	}
	// Same scope and priority: the later expiry wins.
	s = world()
	add(t, s, state.Modifier{Source: "late", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 1, ExpiresTick: 500})
	add(t, s, state.Modifier{Source: "soon", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 0, ExpiresTick: 200})
	if got := effects.Apply(s, effects.StatAirportClosed, 0, lhr); got != 1 {
		t.Errorf("expiry order: %v, want 1", got)
	}
	// Everything equal: the ID decides ("b|…" sorts after "a|…").
	s = world()
	add(t, s, state.Modifier{Source: "b", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 1})
	add(t, s, state.Modifier{Source: "a", Scope: effects.ScopeGlobal, Stat: effects.StatAirportClosed, Op: effects.OpSet, Value: 0})
	if got := effects.Apply(s, effects.StatAirportClosed, 0, lhr); got != 1 {
		t.Errorf("ID order: %v, want 1", got)
	}
}

func TestTargeting(t *testing.T) {
	s := world()
	add(t, s, state.Modifier{Source: "gb", Scope: effects.ScopeCountry, Target: "GB", Stat: effects.StatRevenue, Op: effects.OpMul, Value: 2})
	add(t, s, state.Modifier{Source: "fr", Scope: effects.ScopeCountry, Target: "FR", Stat: effects.StatRevenue, Op: effects.OpMul, Value: 0.5})
	add(t, s, state.Modifier{Source: "all-routes", Scope: effects.ScopeRoute, Stat: effects.StatRevenue, Op: effects.OpAdd, Value: 0.25})
	add(t, s, state.Modifier{Source: "other-stat", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 3})
	if got := effects.Apply(s, effects.StatRevenue, 1, lhr); got != 2.5 { // (1 + 0.25) × 2
		t.Errorf("GB target = %v, want 2.5", got)
	}
	cdg := effects.Target{Airline: "P", Country: "FR", Airport: "CDG"}
	if got := effects.Apply(s, effects.StatRevenue, 1, cdg); got != 0.625 { // (1 + 0.25) × 0.5
		t.Errorf("FR target = %v, want 0.625", got)
	}
	// A target without a route still gets "all routes" (empty Target), not route-specific ones.
	add(t, s, state.Modifier{Source: "rt9", Scope: effects.ScopeRoute, Target: "rt9", Stat: effects.StatRevenue, Op: effects.OpMul, Value: 9})
	if got := effects.Apply(s, effects.StatRevenue, 1, effects.Target{}); got != 1.25 {
		t.Errorf("empty target = %v, want 1.25", got)
	}
	// Airline and plane scopes.
	add(t, s, state.Modifier{Source: "airline", Scope: effects.ScopeAirline, Target: "P", Stat: effects.StatUpkeep, Op: effects.OpMul, Value: 0.8})
	add(t, s, state.Modifier{Source: "plane", Scope: effects.ScopePlane, Target: "pl1", Stat: effects.StatUpkeep, Op: effects.OpMul, Value: 0.5})
	if got := effects.Apply(s, effects.StatUpkeep, 1, lhr); got != 0.5 { // 0.8 × 0.5 = 0.4 → clamped to 0.5
		t.Errorf("airline + plane = %v, want 0.5 (clamped)", got)
	}
	if got := effects.Apply(s, effects.StatUpkeep, 1, effects.Target{Airline: "A1"}); got != 1 {
		t.Errorf("other airline = %v, want 1", got)
	}
}

func TestExpiry(t *testing.T) {
	s := world() // tick 100
	add(t, s, state.Modifier{Source: "ends-now", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 2, ExpiresTick: 100})
	add(t, s, state.Modifier{Source: "ends-next", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 1.5, ExpiresTick: 101})
	add(t, s, state.Modifier{Source: "forever", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 1.25})
	if got := effects.Apply(s, effects.StatDemand, 1, lhr); got != 1.875 { // 1.5 × 1.25; the expired one is ignored
		t.Errorf("active = %v, want 1.875", got)
	}
	removed := effects.Prune(s)
	if !reflect.DeepEqual(removed, []string{"ends-now|demand|global|"}) || len(s.Modifiers) != 2 {
		t.Errorf("prune removed %v, left %d", removed, len(s.Modifiers))
	}
	s.Tick = 101
	if removed := effects.Prune(s); len(removed) != 1 || len(s.Modifiers) != 1 {
		t.Errorf("tick 101: removed %v", removed)
	}
}

func TestSameSourceRefreshesInsteadOfStacking(t *testing.T) {
	s := world()
	m := state.Modifier{Source: "ad", Scope: effects.ScopeAirline, Target: "P", Stat: effects.StatAttractiveness, Op: effects.OpMul, Value: 1.2, ExpiresTick: 200}
	add(t, s, m)
	m.Value, m.ExpiresTick = 1.3, 300
	add(t, s, m)
	if len(s.Modifiers) != 1 {
		t.Fatalf("modifiers = %d, want 1", len(s.Modifiers))
	}
	got := s.Modifiers[effects.Key(m)]
	if got.Value != 1.3 || got.ExpiresTick != 300 || got.ID != "ad|attractiveness|airline|P" {
		t.Errorf("refreshed = %+v", got)
	}
	// A different target is a different modifier.
	m.Target = "A1"
	add(t, s, m)
	if len(s.Modifiers) != 2 {
		t.Errorf("different target replaced the first: %d", len(s.Modifiers))
	}
}

func TestAddValidation(t *testing.T) {
	ok := state.Modifier{Source: "x", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 1}
	cases := map[string]struct {
		change func(*state.Modifier)
		want   error
	}{
		"stat":   {func(m *state.Modifier) { m.Stat = "speed" }, effects.ErrUnknownStat},
		"scope":  {func(m *state.Modifier) { m.Scope = "galaxy" }, effects.ErrUnknownScope},
		"op":     {func(m *state.Modifier) { m.Op = "pow" }, effects.ErrUnknownOp},
		"source": {func(m *state.Modifier) { m.Source = "" }, effects.ErrNoSource},
	}
	for name, tc := range cases {
		s := world()
		m := ok
		tc.change(&m)
		if err := effects.Add(s, m); !errors.Is(err, tc.want) || len(s.Modifiers) != 0 {
			t.Errorf("%s: err = %v, stored %d", name, err, len(s.Modifiers))
		}
	}
	for _, op := range []string{effects.OpAdd, effects.OpMul, effects.OpSet} {
		m := ok
		m.Op = op
		if err := effects.Add(world(), m); err != nil {
			t.Errorf("op %s rejected: %v", op, err)
		}
	}
}

func TestActions(t *testing.T) {
	s := world()
	s.Airlines["P"] = &state.Airline{ID: "P", CashCents: 1000, Reputation: 95}
	s.Planes["pl1"] = &state.Plane{ID: "pl1", Condition: 0.5}

	apply := func(a effects.Action) {
		t.Helper()
		if err := effects.ApplyAction(s, a); err != nil {
			t.Fatal(err)
		}
	}
	apply(effects.Action{Kind: effects.ActionCash, Airline: "P", Cents: -250})
	apply(effects.Action{Kind: effects.ActionReputation, Airline: "P", Value: 10})
	apply(effects.Action{Kind: effects.ActionCondition, Plane: "pl1", Value: 0.75})
	p := s.Airlines["P"]
	if p.CashCents != 750 || p.Reputation != 100 || s.Planes["pl1"].Condition != 1 {
		t.Errorf("after actions: cash %d, reputation %v, condition %v", p.CashCents, p.Reputation, s.Planes["pl1"].Condition)
	}
	apply(effects.Action{Kind: effects.ActionReputation, Airline: "P", Value: -150})
	apply(effects.Action{Kind: effects.ActionCondition, Plane: "pl1", Value: -2})
	if p.Reputation != 0 || s.Planes["pl1"].Condition != 0 {
		t.Errorf("lower clamps: reputation %v, condition %v", p.Reputation, s.Planes["pl1"].Condition)
	}
	errs := map[string]effects.Action{
		"unknown airline (cash)":       {Kind: effects.ActionCash, Airline: "A9"},
		"unknown airline (reputation)": {Kind: effects.ActionReputation, Airline: "A9"},
		"unknown plane":                {Kind: effects.ActionCondition, Plane: "pl9"},
		"unknown kind":                 {Kind: "teleport"},
	}
	wants := map[string]error{
		"unknown airline (cash)": effects.ErrNoAirline, "unknown airline (reputation)": effects.ErrNoAirline,
		"unknown plane": effects.ErrNoPlane, "unknown kind": effects.ErrUnknownAction,
	}
	for name, a := range errs {
		if err := effects.ApplyAction(s, a); !errors.Is(err, wants[name]) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if p.CashCents != 750 {
		t.Error("a failed action changed state")
	}
}

func TestSystemPrunesEveryTick(t *testing.T) {
	s := state.New(1)
	add(t, s, state.Modifier{Source: "x", Scope: effects.ScopeGlobal, Stat: effects.StatDemand, Op: effects.OpMul, Value: 2, ExpiresTick: 3})
	e := engine.New(s, nil, effects.System{})
	e.Advance(2) // tick 2: still active
	if len(s.Modifiers) != 1 {
		t.Fatal("pruned too early")
	}
	e.Advance(1) // tick 3: expired
	if len(s.Modifiers) != 0 {
		t.Fatal("not pruned at its expiry tick")
	}
	if (effects.System{}).Name() != "effects" {
		t.Error("name")
	}
}
