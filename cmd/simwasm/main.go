// Command simwasm is the simulation entry point (ADR-0002).
//
// Compiled to wasm and loaded by the Web Worker, it registers the bridge API as
// globalThis.jetstreamSim and keeps running. Run with the -determinism flag (natively,
// or as wasm under Node), it replays the embedded scripts and prints one line per
// script with its final state hash, for the cross-architecture comparison in CI.
package main

import (
	"fmt"
	"os"

	"github.com/MIcQo/jetstream-tycoon/sim/script"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-determinism" {
		if err := printDeterminism(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	serve() // platform-specific: registers the JS API on wasm
}

func printDeterminism() error {
	all, err := script.All()
	if err != nil {
		return err
	}
	for _, e := range all {
		hashes, err := script.Run(e.Script)
		if err != nil {
			return err
		}
		fmt.Printf("%s %s\n", e.Script.Name, hashes[len(hashes)-1])
	}
	return nil
}
