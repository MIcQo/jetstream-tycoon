package effects

import (
	"errors"
	"fmt"

	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// Action kinds: instant effects (ADR-0005).
const (
	ActionCash       = "cash"       // add Cents to the airline's cash (may be negative)
	ActionReputation = "reputation" // add Value to reputation, clamped to 0..100
	ActionCondition  = "condition"  // add Value to the plane's condition, clamped to 0..1
)

// Action is an instant change.
type Action struct {
	Kind    string
	Airline state.AirlineID
	Plane   state.ID
	Cents   int64
	Value   float64
}

// Action errors.
var (
	ErrUnknownAction = errors.New("effects: unknown action")
	ErrNoAirline     = errors.New("effects: airline not found")
	ErrNoPlane       = errors.New("effects: plane not found")
)

// ApplyAction performs a. It changes nothing if it returns an error.
func ApplyAction(s *state.State, a Action) error {
	switch a.Kind {
	case ActionCash, ActionReputation:
		al := s.Airlines[a.Airline]
		if al == nil {
			return fmt.Errorf("%w: %q", ErrNoAirline, a.Airline)
		}
		if a.Kind == ActionCash {
			al.CashCents += a.Cents
		} else {
			al.Reputation = min(max(al.Reputation+a.Value, 0), 100)
		}
	case ActionCondition:
		p := s.Planes[a.Plane]
		if p == nil {
			return fmt.Errorf("%w: %q", ErrNoPlane, a.Plane)
		}
		p.Condition = min(max(p.Condition+a.Value, 0), 1)
	default:
		return fmt.Errorf("%w %q", ErrUnknownAction, a.Kind)
	}
	return nil
}
