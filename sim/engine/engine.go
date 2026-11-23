// Package engine runs the deterministic tick loop (TECH_DESIGN §3): queued
// commands are applied at the tick boundary, then every registered System runs in
// its fixed order.
package engine

import (
	"github.com/MIcQo/jetstream-tycoon/sim/command"
	"github.com/MIcQo/jetstream-tycoon/sim/rng"
	"github.com/MIcQo/jetstream-tycoon/sim/state"
)

// System is one game mechanic. Systems run in registration order every tick, and
// OnDay runs (after OnTick) on the first tick of each game day.
type System interface {
	Name() string
	OnTick(c *TickCtx)
	OnDay(c *TickCtx)
}

// Notification is a message for the player, as an i18n key plus raw parameters
// that the UI formats (ADR-0016).
type Notification struct {
	Key    string            `json:"key"`
	Params map[string]string `json:"params,omitempty"`
}

// TickCtx gives a system access to the state and to deterministic services.
type TickCtx struct {
	State         *state.State
	system        string
	notifications *[]Notification
}

// RNG returns the calling system's random stream for this tick. Extra keys (such
// as an airline ID) give independent sub-streams.
func (c *TickCtx) RNG(extra ...string) *rng.Stream {
	return rng.For(c.State.Seed, c.system, c.State.Tick, extra...)
}

// Notify queues a notification for the UI.
func (c *TickCtx) Notify(key string, params map[string]string) {
	*c.notifications = append(*c.notifications, Notification{Key: key, Params: params})
}

// Rejection records a command that failed validation.
type Rejection struct {
	Type     string `json:"type"`
	ErrorKey string `json:"errorKey"`
}

// Output is everything produced by Advance.
type Output struct {
	Notifications []Notification `json:"notifications"`
	Rejected      []Rejection    `json:"rejected"`
}

// Engine owns the state, the ordered systems and the command queue.
type Engine struct {
	State   *state.State
	systems []System
	queue   []command.Command
}

// New creates an engine. Systems run in the given order.
func New(s *state.State, systems ...System) *Engine {
	return &Engine{State: s, systems: systems}
}

// Enqueue adds a command to be applied, in FIFO order, at the next tick boundary.
func (e *Engine) Enqueue(c command.Command) {
	e.queue = append(e.queue, c)
}

// Advance runs n ticks.
func (e *Engine) Advance(n int) Output {
	var out Output
	for i := 0; i < n; i++ {
		e.applyQueued(&out)
		e.State.Tick++
		dayStart := e.State.Tick%state.TicksPerDay == 0
		for _, sys := range e.systems {
			ctx := &TickCtx{State: e.State, system: sys.Name(), notifications: &out.Notifications}
			sys.OnTick(ctx)
			if dayStart {
				sys.OnDay(ctx)
			}
		}
	}
	return out
}

func (e *Engine) applyQueued(out *Output) {
	for _, c := range e.queue {
		if err := c.Validate(e.State); err != nil {
			key := "errors.cmd.unknown"
			if ce, ok := err.(*command.Error); ok { //nolint:errorlint // command errors are never wrapped
				key = ce.Key
			}
			out.Rejected = append(out.Rejected, Rejection{Type: c.Type(), ErrorKey: key})
			continue
		}
		c.Apply(e.State)
	}
	e.queue = e.queue[:0]
}
