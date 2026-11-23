package command_test

import (
	"errors"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

func TestSetSpeed(t *testing.T) {
	s := state.New(1)
	for _, v := range []int{0, 1, 2, 4} {
		c := command.SetSpeed{Speed: v}
		if err := c.Validate(s); err != nil {
			t.Errorf("SetSpeed(%d).Validate = %v", v, err)
		}
		c.Apply(s)
		if s.Speed != v {
			t.Errorf("after Apply(%d) speed = %d", v, s.Speed)
		}
	}
	for _, v := range []int{-1, 3, 5, 8} {
		err := command.SetSpeed{Speed: v}.Validate(s)
		if !errors.Is(err, command.ErrInvalidSpeed) {
			t.Errorf("SetSpeed(%d).Validate = %v, want ErrInvalidSpeed", v, err)
		}
	}
	if got := (command.SetSpeed{}).Type(); got != "setSpeed" {
		t.Errorf("Type() = %q", got)
	}
}

func TestErrorMessageIsKey(t *testing.T) {
	if got := command.ErrInvalidSpeed.Error(); got != "errors.cmd.invalid_speed" {
		t.Errorf("Error() = %q", got)
	}
}
