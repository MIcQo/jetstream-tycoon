// Package command defines state-changing commands (ADR-0020). Every change to the
// game state, from the player, an AI airline or an ad reward, is a Command that is
// validated and then applied at a tick boundary.
package command

import "github.com/MIcQo/jetstream-tycoon/sim/state"

// Command is a validated state change.
type Command interface {
	// Type is the stable wire name. It must never change once shipped.
	Type() string
	// Validate reports whether the command can be applied to s, without changing s.
	Validate(s *state.State) error
	// Apply changes s. It is only called after Validate returned nil.
	Apply(s *state.State)
}

// Error is a validation failure carrying an i18n key for the UI (ADR-0016).
type Error struct {
	Key string
}

func (e *Error) Error() string { return e.Key }

// ErrInvalidSpeed is returned for a speed other than 0, 1, 2 or 4.
var ErrInvalidSpeed = &Error{Key: "errors.cmd.invalid_speed"}

// SetSpeed changes the game speed: 0 = paused, 1, 2 or 4 = multiplier.
type SetSpeed struct {
	Speed int `json:"speed"`
}

// Type implements Command.
func (SetSpeed) Type() string { return "setSpeed" }

// Validate implements Command.
func (c SetSpeed) Validate(*state.State) error {
	switch c.Speed {
	case 0, 1, 2, 4:
		return nil
	}
	return ErrInvalidSpeed
}

// Apply implements Command.
func (c SetSpeed) Apply(s *state.State) { s.Speed = c.Speed }
