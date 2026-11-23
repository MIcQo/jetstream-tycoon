package bridge_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/bridge"
)

func decode[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad JSON %q: %v", s, err)
	}
	return v
}

func TestNewGameAndAdvance(t *testing.T) {
	var s bridge.Session
	v := decode[bridge.View](t, must(t)(s.NewGame(`{"seed":42}`)))
	if v.Tick != 0 || v.Day != 0 || v.Speed != 1 || v.CashCents != 100_000_00 {
		t.Fatalf("initial view = %+v", v)
	}

	r := decode[bridge.AdvanceResult](t, must(t)(s.Advance(24)))
	if r.View.Tick != 24 || r.View.Day != 1 || r.View.CashCents <= v.CashCents {
		t.Fatalf("after 24 ticks: %+v", r.View)
	}
	if len(r.Notifications) != 1 || r.Notifications[0].Key != "notifications.day.started" {
		t.Fatalf("notifications = %+v", r.Notifications)
	}

	// A second session with the same seed is identical (deterministic).
	var s2 bridge.Session
	must(t)(s2.NewGame(`{"seed":42}`))
	r2 := decode[bridge.AdvanceResult](t, must(t)(s2.Advance(24)))
	if r2.View != r.View {
		t.Fatalf("same seed, different view: %+v vs %+v", r2.View, r.View)
	}
}

func TestCommands(t *testing.T) {
	var s bridge.Session
	if got := s.Command(`{"type":"setSpeed","speed":2}`); got != `{"ok":false,"errorKey":"errors.sim.not_started"}` {
		t.Fatalf("command before start = %s", got)
	}
	must(t)(s.NewGame(`{"seed":1}`))

	cases := map[string]string{
		`{"type":"setSpeed","speed":4}`: `{"ok":true}`,
		`{"type":"setSpeed","speed":3}`: `{"ok":false,"errorKey":"errors.cmd.invalid_speed"}`,
		`{"type":"warp"}`:               `{"ok":false,"errorKey":"errors.cmd.unknown_type"}`,
		`{oops`:                         `{"ok":false,"errorKey":"errors.cmd.malformed"}`,
	}
	for in, want := range cases {
		if got := s.Command(in); got != want {
			t.Errorf("Command(%s) = %s, want %s", in, got, want)
		}
	}

	// Accepted commands apply at the next boundary, even with Advance(0) while paused.
	if v := decode[bridge.AdvanceResult](t, must(t)(s.Advance(0))).View; v.Speed != 4 || v.Tick != 0 {
		t.Fatalf("after Advance(0): %+v", v)
	}
}

func TestErrors(t *testing.T) {
	var s bridge.Session
	if _, err := s.Advance(1); !errors.Is(err, bridge.ErrNotStarted) {
		t.Errorf("Advance before start = %v", err)
	}
	if _, err := s.NewGame(`nope`); err == nil || !strings.Contains(err.Error(), "newGame") {
		t.Errorf("bad newGame request = %v", err)
	}
	must(t)(s.NewGame(`{"seed":1}`))
	if _, err := s.Advance(-1); err == nil {
		t.Error("negative ticks accepted")
	}
}

func must(t *testing.T) func(string, error) string {
	return func(s string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
}

func TestAdvanceReturnsEmptyArraysNotNull(t *testing.T) {
	var s bridge.Session
	must(t)(s.NewGame(`{"seed":1}`))
	out := must(t)(s.Advance(1))
	if !strings.Contains(out, `"notifications":[]`) || !strings.Contains(out, `"rejected":[]`) {
		t.Fatalf("Advance(1) = %s, want empty arrays", out)
	}
}

func TestGuard(t *testing.T) {
	if got := bridge.Guard(func() (string, error) { return `{"ok":true}`, nil }); got != `{"ok":true}` {
		t.Errorf("success = %s", got)
	}
	if got := bridge.Guard(func() (string, error) { return "", errors.New(`bad "input"`) }); got != `{"error":"bad \"input\""}` {
		t.Errorf("error = %s", got)
	}
	if got := bridge.Guard(func() (string, error) { panic("kaboom") }); got != `{"error":"panic: kaboom"}` {
		t.Errorf("panic = %s", got)
	}
}
