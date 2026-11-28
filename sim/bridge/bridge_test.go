package bridge_test

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
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
	v := decode[bridge.View](t, must(t)(s.NewGame(`{"seed":42,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`)))
	if v.Tick != 0 || v.Day != 0 || v.Speed != 1 || v.CashCents <= 0 || v.AirlineName != "Test Air" || v.HomeCountry != "GB" || !strings.HasPrefix(v.ContentVersion, "c") {
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
	must(t)(s2.NewGame(`{"seed":42,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`))
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
	must(t)(s.NewGame(`{"seed":1,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`))

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
	must(t)(s.NewGame(`{"seed":1,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`))
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
	must(t)(s.NewGame(`{"seed":1,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`))
	out := must(t)(s.Advance(1))
	if !strings.Contains(out, `"notifications":[]`) || !strings.Contains(out, `"rejected":[]`) || !strings.Contains(out, `"reports":[]`) {
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
	got := bridge.Guard(func() (string, error) { panic("kaboom") })
	var e struct{ Error string }
	if err := json.Unmarshal([]byte(got), &e); err != nil || !strings.HasPrefix(e.Error, "panic: kaboom\n") ||
		!strings.Contains(e.Error, "goroutine") || len(e.Error) > 4200 {
		t.Errorf("panic = %.200s (len %d)", got, len(got))
	}
}

func TestReportsAreReturnedOnceAndBounded(t *testing.T) {
	var s bridge.Session
	s.ReportNonFinite("before_start", 1) // no game yet: tick 0
	must(t)(s.NewGame(`{"seed":1,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`))
	first := decode[bridge.AdvanceResult](t, must(t)(s.Advance(3)))
	if want := []bridge.Report{{Kind: "nonFinite", Context: "before_start", Detail: "1", Tick: 0}}; !reflect.DeepEqual(first.Reports, want) {
		t.Fatalf("first reports = %+v, want %+v", first.Reports, want)
	}
	s.ReportNonFinite("fuel_price", math.Inf(1))

	r := decode[bridge.AdvanceResult](t, must(t)(s.Advance(1)))
	if want := []bridge.Report{{Kind: "nonFinite", Context: "fuel_price", Detail: "+Inf", Tick: 3}}; !reflect.DeepEqual(r.Reports, want) {
		t.Fatalf("reports = %+v, want %+v", r.Reports, want)
	}
	if again := decode[bridge.AdvanceResult](t, must(t)(s.Advance(1))); len(again.Reports) != 0 {
		t.Fatalf("reports not cleared: %+v", again.Reports)
	}

	for i := 0; i < 60; i++ {
		s.ReportNonFinite("loop", math.NaN())
	}
	if n := len(decode[bridge.AdvanceResult](t, must(t)(s.Advance(1))).Reports); n != 50 {
		t.Fatalf("got %d reports, want the cap of 50", n)
	}
}

func TestNewGameValidationErrorKeys(t *testing.T) {
	var s bridge.Session
	got := bridge.Guard(func() (string, error) {
		return s.NewGame(`{"seed":1,"airlineName":"X","homeCountry":"ZZ","difficulty":"normal"}`)
	})
	if got != `{"error":"errors.newgame.home_country"}` {
		t.Errorf("unknown home country = %s", got)
	}
	if _, err := s.Advance(1); !errors.Is(err, bridge.ErrNotStarted) {
		t.Error("a failed newGame must not start a session")
	}
}
