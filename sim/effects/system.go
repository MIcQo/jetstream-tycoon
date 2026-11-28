package effects

import "github.com/MIcQo/jetstream-tycoon/sim/engine"

// System prunes expired modifiers. It runs first, so every later system sees only
// modifiers in force this tick.
type System struct{}

// Name implements engine.System.
func (System) Name() string { return "effects" }

// OnTick implements engine.System.
func (System) OnTick(c *engine.TickCtx) { Prune(c.State) }

// OnDay implements engine.System.
func (System) OnDay(*engine.TickCtx) {}
