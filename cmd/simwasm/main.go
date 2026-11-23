// Command simwasm is the WebAssembly bridge entrypoint for the simulation (ADR-0002).
package main

import (
	"fmt"

	"github.com/MIcQo/jetstream-tycoon/sim"
)

func main() {
	s := sim.New(42)
	s.Advance(24)
	fmt.Println(s.Hash())
}
