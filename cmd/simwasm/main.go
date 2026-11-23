// Command simwasm is the WebAssembly bridge entrypoint for the simulation (ADR-0002).
// Until the bridge lands (M0.6) it prints the state hash of the scripted
// determinism game, which CI compares across amd64, arm64 and wasm.
package main

import (
	"fmt"

	"github.com/MIcQo/jetstream-tycoon/sim"
	"github.com/MIcQo/jetstream-tycoon/sim/command"
)

func main() {
	e := sim.NewGame(42)
	e.Advance(24)
	e.Enqueue(command.SetSpeed{Speed: 4})
	e.Advance(24 * 6)
	fmt.Println(e.State.Hash())
}
