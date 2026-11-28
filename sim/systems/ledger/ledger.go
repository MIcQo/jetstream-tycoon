// Package ledger is the M0 stub system. It exercises the tick loop, the per-system
// RNG stream, detmath rounding and day notifications, and will be replaced by the
// real Economy system in M3.
package ledger

import (
	"strconv"

	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
	"github.com/MIcQo/jetstream-tycoon/sim/engine"
)

// System adds a small random amount of cash to the player every tick and announces each new day.
type System struct{}

// Name implements engine.System.
func (System) Name() string { return "ledger" }

// OnTick implements engine.System.
func (System) OnTick(c *engine.TickCtx) {
	if p := c.State.Player(); p != nil {
		p.CashCents += detmath.RoundCents(c.RNG().Float64() * 100)
	}
}

// OnDay implements engine.System.
func (System) OnDay(c *engine.TickCtx) {
	c.Notify("notifications.day.started", map[string]string{"day": strconv.FormatInt(c.State.Day(), 10)})
}
