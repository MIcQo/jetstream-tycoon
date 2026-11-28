// Package sim wires the deterministic game simulation together (ADR-0002, ADR-0006).
package sim

import (
	"github.com/MIcQo/jetstream-tycoon/sim/engine"
	"github.com/MIcQo/jetstream-tycoon/sim/systems/ledger"
)

// Systems returns the registered systems in their fixed execution order
// (TECH_DESIGN §3). The order is part of the game rules: changing it changes results.
func Systems() []engine.System {
	return []engine.System{
		ledger.System{},
	}
}
