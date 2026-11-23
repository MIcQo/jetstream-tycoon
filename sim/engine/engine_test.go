package engine_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/engine"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// recorder logs every call so tests can check order and timing.
type recorder struct {
	name string
	log  *[]string
}

func (r recorder) Name() string { return r.name }
func (r recorder) OnTick(c *engine.TickCtx) {
	*r.log = append(*r.log, fmt.Sprintf("%s.tick@%d", r.name, c.State.Tick))
}
func (r recorder) OnDay(c *engine.TickCtx) {
	*r.log = append(*r.log, fmt.Sprintf("%s.day@%d", r.name, c.State.Tick))
	c.Notify("notifications.test", map[string]string{"system": r.name})
}

func TestSystemsRunInOrderAndDaysFire(t *testing.T) {
	var log []string
	e := engine.New(state.New(1), recorder{"a", &log}, recorder{"b", &log})
	out := e.Advance(24)

	if e.State.Tick != 24 {
		t.Fatalf("tick = %d", e.State.Tick)
	}
	want := []string{"a.tick@1", "b.tick@1"}
	if !reflect.DeepEqual(log[:2], want) {
		t.Fatalf("first calls = %v, want %v", log[:2], want)
	}
	tail := log[len(log)-4:]
	wantTail := []string{"a.tick@24", "a.day@24", "b.tick@24", "b.day@24"}
	if !reflect.DeepEqual(tail, wantTail) {
		t.Fatalf("day-boundary calls = %v, want %v", tail, wantTail)
	}
	if len(log) != 24*2+2 {
		t.Fatalf("%d calls, want %d", len(log), 24*2+2)
	}
	if len(out.Notifications) != 2 || out.Notifications[0].Params["system"] != "a" || out.Notifications[1].Key != "notifications.test" {
		t.Fatalf("notifications = %+v", out.Notifications)
	}
}

type badCmd struct{}

func (badCmd) Type() string                { return "bad" }
func (badCmd) Validate(*state.State) error { return errors.New("plain error") }
func (badCmd) Apply(*state.State)          { panic("must not apply") }

func TestCommandsApplyAtTickBoundaryInOrder(t *testing.T) {
	var seen []int
	probe := probeSystem{seen: &seen}
	e := engine.New(state.New(1), probe)

	e.Enqueue(command.SetSpeed{Speed: 2})
	e.Enqueue(command.SetSpeed{Speed: 4})
	e.Enqueue(command.SetSpeed{Speed: 3}) // invalid
	e.Enqueue(badCmd{})
	if e.State.Speed != 1 {
		t.Fatal("command applied before Advance")
	}
	out := e.Advance(2)

	if seen[0] != 4 || seen[1] != 4 {
		t.Fatalf("systems saw speeds %v, want last valid command (4) applied before the first tick", seen)
	}
	wantRejected := []engine.Rejection{
		{Type: "setSpeed", ErrorKey: "errors.cmd.invalid_speed"},
		{Type: "bad", ErrorKey: "errors.cmd.unknown"},
	}
	if !reflect.DeepEqual(out.Rejected, wantRejected) {
		t.Fatalf("rejected = %+v, want %+v", out.Rejected, wantRejected)
	}

	// Advance(0) applies commands without running a tick (paused game).
	e.Enqueue(command.SetSpeed{Speed: 0})
	if paused := e.Advance(0); len(paused.Rejected) != 0 || e.State.Speed != 0 || e.State.Tick != 2 {
		t.Fatalf("Advance(0): speed=%d tick=%d rejected=%v", e.State.Speed, e.State.Tick, paused.Rejected)
	}

	// The queue is emptied: advancing again rejects nothing.
	if again := e.Advance(1); len(again.Rejected) != 0 {
		t.Fatalf("queue not cleared: %+v", again.Rejected)
	}
}

type probeSystem struct{ seen *[]int }

func (p probeSystem) Name() string             { return "probe" }
func (p probeSystem) OnTick(c *engine.TickCtx) { *p.seen = append(*p.seen, c.State.Speed) }
func (probeSystem) OnDay(*engine.TickCtx)      {}

type rngSystem struct {
	name string
	got  *[]uint64
}

func (r rngSystem) Name() string { return r.name }
func (r rngSystem) OnTick(c *engine.TickCtx) {
	*r.got = append(*r.got, c.RNG().Uint64(), c.RNG("A1").Uint64())
}
func (rngSystem) OnDay(*engine.TickCtx) {}

func TestRNGStreamsPerSystemAndKey(t *testing.T) {
	var a, b []uint64
	engine.New(state.New(5), rngSystem{"a", &a}, rngSystem{"b", &b}).Advance(2)
	if a[0] == b[0] {
		t.Error("two systems got the same stream")
	}
	if a[0] == a[1] {
		t.Error("extra key did not change the stream")
	}
	if a[0] == a[2] {
		t.Error("consecutive ticks got the same stream")
	}
	var again []uint64
	engine.New(state.New(5), rngSystem{"a", &again}).Advance(2)
	if !reflect.DeepEqual(a, again) {
		t.Errorf("stream depends on other systems: %v vs %v", a, again)
	}
}
