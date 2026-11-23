package command_test

import (
	"errors"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/command"
)

func TestDecode(t *testing.T) {
	c, err := command.Decode([]byte(`{"type":"setSpeed","speed":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := c.(command.SetSpeed); !ok || got.Speed != 2 {
		t.Fatalf("Decode = %#v", c)
	}
}

func TestDecodeErrors(t *testing.T) {
	cases := map[string]error{
		`{"type":"launchRockets"}`:           command.ErrUnknownType,
		`{"speed":2}`:                        command.ErrUnknownType,
		`not json`:                           command.ErrMalformed,
		`{"type":"setSpeed","speed":"fast"}`: command.ErrMalformed,
	}
	for in, want := range cases {
		if _, err := command.Decode([]byte(in)); !errors.Is(err, want) {
			t.Errorf("Decode(%s) = %v, want %v", in, err, want)
		}
	}
}
