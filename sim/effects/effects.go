// Package effects is the single mechanism for bonuses and penalties (ADR-0005,
// ADR-0021). Events, investments, seasons, upgrades and ad rewards all add
// Modifiers (temporary or permanent changes to a stat) or apply Actions (instant
// changes such as cash). Systems read stats only through Apply.
package effects

import (
	"errors"
	"fmt"
	"slices"

	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// Scopes, in stacking order (TECH_DESIGN §6, step 1).
const (
	ScopeGlobal  = "global"
	ScopeAirline = "airline"
	ScopeCountry = "country"
	ScopeAirport = "airport"
	ScopeRoute   = "route"
	ScopePlane   = "plane"
)

var scopeRank = map[string]int{
	ScopeGlobal: 0, ScopeAirline: 1, ScopeCountry: 2, ScopeAirport: 3, ScopeRoute: 4, ScopePlane: 5,
}

// Operations.
const (
	OpAdd = "add"
	OpMul = "mul"
	OpSet = "set"
)

// Target identifies what a stat is evaluated for: the entity and its parents
// (plane → route → airport → country → airline). Empty fields don't match
// scoped modifiers with a specific target.
type Target struct {
	Airline string
	Country string
	Airport string
	Route   string
	Plane   string
}

func (t Target) id(scope string) string {
	switch scope {
	case ScopeAirline:
		return t.Airline
	case ScopeCountry:
		return t.Country
	case ScopeAirport:
		return t.Airport
	case ScopeRoute:
		return t.Route
	case ScopePlane:
		return t.Plane
	}
	return ""
}

// Validation errors for Add.
var (
	ErrUnknownStat  = errors.New("effects: unknown stat")
	ErrUnknownScope = errors.New("effects: unknown scope")
	ErrUnknownOp    = errors.New("effects: unknown op")
	ErrNoSource     = errors.New("effects: modifier needs a source")
)

// Key is the modifier's identity under the same-source rule: one modifier per
// (source, stat, scope, target). Adding another with the same key replaces it.
func Key(m state.Modifier) string {
	return m.Source + "|" + m.Stat + "|" + m.Scope + "|" + m.Target
}

// Add validates m and stores it, replacing any modifier with the same Key (the
// same-source rule: re-applying refreshes value and expiry, it doesn't stack).
func Add(s *state.State, m state.Modifier) error {
	if _, ok := Stats[m.Stat]; !ok {
		return fmt.Errorf("%w %q", ErrUnknownStat, m.Stat)
	}
	if _, ok := scopeRank[m.Scope]; !ok {
		return fmt.Errorf("%w %q", ErrUnknownScope, m.Scope)
	}
	switch m.Op {
	case OpAdd, OpMul, OpSet:
	default:
		return fmt.Errorf("%w %q", ErrUnknownOp, m.Op)
	}
	if m.Source == "" {
		return ErrNoSource
	}
	m.ID = Key(m)
	s.Modifiers[m.ID] = &m
	return nil
}

// active reports whether m is in force at tick (0 = permanent).
func active(m *state.Modifier, tick int64) bool {
	return m.ExpiresTick == 0 || m.ExpiresTick > tick
}

// matches reports whether m applies to target t.
func matches(m *state.Modifier, stat string, t Target) bool {
	if m.Stat != stat {
		return false
	}
	return m.Scope == ScopeGlobal || m.Target == "" || m.Target == t.id(m.Scope)
}

// Apply returns stat for target t at the current tick (TECH_DESIGN §6):
//
//  1. collect active modifiers that match, sorted by (scope rank, priority,
//     expiry, id) — float arithmetic is order-sensitive, so the order is fixed;
//  2. v = (base + Σ add) × Π mul;
//  3. clamp v to the stat's range;
//  4. the last "set" in sorted order, if any, replaces v.
func Apply(s *state.State, stat string, base float64, t Target) float64 {
	var ms []*state.Modifier
	for _, id := range state.SortedKeys(s.Modifiers) {
		if m := s.Modifiers[id]; active(m, s.Tick) && matches(m, stat, t) {
			ms = append(ms, m)
		}
	}
	slices.SortStableFunc(ms, compare)

	sum, product := base, 1.0
	var set *state.Modifier
	for _, m := range ms {
		switch m.Op {
		case OpAdd:
			sum += m.Value
		case OpMul:
			product *= m.Value
		case OpSet:
			set = m
		}
	}
	v := float64(sum * product)
	if r, ok := Stats[stat]; ok {
		v = min(max(v, r.Min), r.Max)
	}
	if set != nil {
		v = set.Value
	}
	return v
}

func compare(a, b *state.Modifier) int {
	if d := scopeRank[a.Scope] - scopeRank[b.Scope]; d != 0 {
		return d
	}
	if d := a.Priority - b.Priority; d != 0 {
		return d
	}
	if a.ExpiresTick != b.ExpiresTick {
		if a.ExpiresTick < b.ExpiresTick {
			return -1
		}
		return 1
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}

// Prune removes modifiers that have expired by tick (TECH_DESIGN §6: at the first
// tick ≥ expiresTick). It returns the removed IDs in order.
func Prune(s *state.State) []string {
	var removed []string
	for _, id := range state.SortedKeys(s.Modifiers) {
		if !active(s.Modifiers[id], s.Tick) {
			delete(s.Modifiers, id)
			removed = append(removed, id)
		}
	}
	return removed
}
