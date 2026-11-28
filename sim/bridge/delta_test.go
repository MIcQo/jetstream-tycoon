package bridge_test

import (
	"bytes"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/MIcQo/jetstream-tycoon/sim/bridge"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

type wireDelta struct {
	Full    bool                                 `msgpack:"full"`
	Tick    int64                                `msgpack:"tick"`
	Changed map[string]map[string]map[string]any `msgpack:"changed"`
	Deleted map[string][]string                  `msgpack:"deleted"`
	Flights []bridge.Flight                      `msgpack:"flights"`
}

func unpack(t *testing.T) func([]byte, error) wireDelta {
	t.Helper()
	return func(b []byte, err error) wireDelta {
		t.Helper()
		return unpackBytes(t, b, err)
	}
}

func unpackBytes(t *testing.T, b []byte, err error) wireDelta {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	dec := msgpack.NewDecoder(bytes.NewReader(b))
	dec.SetCustomStructTag("json")
	var d wireDelta
	if err := dec.Decode(&d); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	return d
}

func ids(m map[string]map[string]any) map[string]bool {
	out := map[string]bool{}
	for id := range m {
		out[id] = true
	}
	return out
}

func world() *state.State {
	s := state.New(1)
	s.Airlines[state.PlayerID] = &state.Airline{ID: state.PlayerID, Name: "P", CashCents: 100}
	s.Countries["GB"] = &state.Country{ISO2: "GB", OwnerID: state.PlayerID}
	s.Modifiers["m1"] = &state.Modifier{ID: "m1", Stat: "demand", Op: "mul", Value: 1.1}
	return s
}

func TestTrackerSnapshotAndChanges(t *testing.T) {
	s := world()
	var tr bridge.Tracker

	full := unpack(t)(tr.Snapshot(s))
	if !full.Full || len(full.Deleted) != 0 {
		t.Fatalf("snapshot: full=%v deleted=%v", full.Full, full.Deleted)
	}
	for _, c := range []string{"airlines", "countries", "modifiers", "world"} {
		if len(full.Changed[c]) == 0 {
			t.Errorf("snapshot has no %s", c)
		}
	}
	if got := full.Changed["airlines"]["P"]["cashCents"]; got != int8(100) {
		t.Errorf("airline cash = %#v", got)
	}

	if d := unpack(t)(tr.Next(s)); d.Full || len(d.Changed) != 0 || len(d.Deleted) != 0 {
		t.Fatalf("no change produced %+v", d)
	}

	s.Tick = 5
	s.Airlines[state.PlayerID].CashCents = 250
	delete(s.Modifiers, "m1")
	s.Planes["pl1"] = &state.Plane{ID: "pl1", AirlineID: state.PlayerID, State: state.PlaneFlying, FlightFrom: "LHR", FlightTo: "CDG", DepartTick: 4, BusyUntil: 6}
	s.Planes["pl2"] = &state.Plane{ID: "pl2", AirlineID: state.PlayerID, State: state.PlaneIdle}
	s.Fuel.PriceCents = 90

	d := unpack(t)(tr.Next(s))
	if d.Full || d.Tick != 5 {
		t.Fatalf("delta header: %+v", d)
	}
	want := map[string]map[string]bool{"airlines": {"P": true}, "planes": {"pl1": true, "pl2": true}, "world": {"world": true}}
	got := map[string]map[string]bool{}
	for c, m := range d.Changed {
		got[c] = ids(m)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changed = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(d.Deleted, map[string][]string{"modifiers": {"m1"}}) {
		t.Errorf("deleted = %v", d.Deleted)
	}
	if want := []bridge.Flight{{PlaneID: "pl1", From: "LHR", To: "CDG", DepartTick: 4, ArriveTick: 6}}; !reflect.DeepEqual(d.Flights, want) {
		t.Errorf("flights = %+v", d.Flights)
	}

	// A snapshot resets the baseline: everything is sent again, then nothing.
	if full := unpack(t)(tr.Snapshot(s)); len(full.Changed["planes"]) != 2 || len(full.Changed["modifiers"]) != 0 {
		t.Errorf("second snapshot: %v", full.Changed)
	}
	if d := unpack(t)(tr.Next(s)); len(d.Changed) != 0 {
		t.Errorf("after snapshot: %v", d.Changed)
	}
}

func TestTrackerFirstNextIsFull(t *testing.T) {
	var tr bridge.Tracker
	if d := unpack(t)(tr.Next(world())); !d.Full || len(d.Changed["airlines"]) != 1 {
		t.Fatalf("first Next = %+v", d)
	}
}

func TestTrackerIsDeterministic(t *testing.T) {
	var a, b bridge.Tracker
	x, _ := a.Snapshot(world())
	y, _ := b.Snapshot(world())
	if !bytes.Equal(x, y) {
		t.Error("same state, different snapshot bytes")
	}
}

func TestSessionSnapshotAndDelta(t *testing.T) {
	var s bridge.Session
	if _, err := s.Snapshot(); !errors.Is(err, bridge.ErrNotStarted) {
		t.Errorf("snapshot before start: %v", err)
	}
	if _, err := s.Delta(); !errors.Is(err, bridge.ErrNotStarted) {
		t.Errorf("delta before start: %v", err)
	}
	must(t)(s.NewGame(`{"seed":42,"airlineName":"Test Air","homeCountry":"GB","difficulty":"normal"}`))
	full := unpack(t)(s.Snapshot())
	if full.Changed["airlines"]["P"]["name"] != "Test Air" || len(full.Changed["planes"]) != 1 {
		t.Fatalf("snapshot = %v", full.Changed)
	}
	must(t)(s.Advance(24))
	if d := unpack(t)(s.Delta()); d.Full || d.Tick != 24 || len(d.Changed["airlines"]) != 1 || len(d.Changed["planes"]) != 0 {
		t.Errorf("delta after a day = %+v", d)
	}
	// A new game resets the baseline, so the next delta is full.
	must(t)(s.NewGame(`{"seed":1,"airlineName":"Other","homeCountry":"JP","difficulty":"easy"}`))
	if d := unpack(t)(s.Delta()); !d.Full {
		t.Error("delta after new game is not full")
	}
	data, _ := s.Save()
	must(t)(s.Load(data))
	if d := unpack(t)(s.Delta()); !d.Full {
		t.Error("delta after load is not full")
	}
}

func TestGuardBytes(t *testing.T) {
	if b, e := bridge.GuardBytes(func() ([]byte, error) { return []byte{1}, nil }); e != "" || !bytes.Equal(b, []byte{1}) {
		t.Errorf("ok = %v %q", b, e)
	}
	if b, e := bridge.GuardBytes(func() ([]byte, error) { return []byte{1}, errors.New("boom") }); b != nil || e != `{"error":"boom"}` {
		t.Errorf("error = %v %q", b, e)
	}
	if b, e := bridge.GuardBytes(func() ([]byte, error) { panic("x") }); b != nil || e == "" {
		t.Errorf("panic = %v %q", b, e)
	}
}

// BenchmarkTrackerNext measures one UI pulse on a late-game state: 7 airlines
// with 500 planes, 400 routes, 1000 markets and 300 modifiers in all.
func BenchmarkTrackerNext(b *testing.B) {
	s := world()
	for i := range 1000 {
		id := state.ID("x" + strconv.Itoa(i))
		s.Markets[string(id)] = &state.Market{Key: string(id), Background: []state.BackgroundCarrier{{ID: "bg", SeatsPerDay: 300}},
			LastShares: map[state.AirlineID]float64{"P": 0.4, "A1": 0.3, "A2": 0.3}}
		if i < 500 {
			s.Planes[id] = &state.Plane{ID: id, ModelID: "a320", State: state.PlaneFlying, FlightFrom: "LHR", FlightTo: "CDG"}
		}
		if i < 400 {
			s.Routes[id] = &state.Route{ID: id, From: "LHR", To: "CDG", PlaneIDs: []state.ID{id}}
		}
		if i < 300 {
			s.Modifiers[string(id)] = &state.Modifier{ID: string(id), Stat: "demand", Op: "mul", Value: 1.1}
		}
	}
	var tr bridge.Tracker
	if _, err := tr.Snapshot(s); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := range b.N {
		s.Airlines[state.PlayerID].CashCents = int64(i)
		if _, err := tr.Next(s); err != nil {
			b.Fatal(err)
		}
	}
}

// Applying every delta to the first snapshot gives the same world as a fresh
// snapshot at the end: nothing changed is missed and nothing stale is kept.
func TestDeltasRebuildTheSnapshot(t *testing.T) {
	var s bridge.Session
	must(t)(s.NewGame(`{"seed":5,"airlineName":"Merge Air","homeCountry":"US","difficulty":"hard"}`))
	world := unpack(t)(s.Snapshot()).Changed
	for i := range 40 {
		must(t)(s.Advance(i % 7))
		d := unpack(t)(s.Delta())
		for c, items := range d.Changed {
			if world[c] == nil {
				world[c] = map[string]map[string]any{}
			}
			for id, v := range items {
				world[c][id] = v
			}
		}
		for c, gone := range d.Deleted {
			for _, id := range gone {
				delete(world[c], id)
			}
		}
	}
	if fresh := unpack(t)(s.Snapshot()).Changed; !reflect.DeepEqual(world, fresh) {
		t.Errorf("merged deltas differ from a fresh snapshot:\n%v\n%v", world, fresh)
	}
}
