//go:build !(js && wasm)

package main

import (
	"fmt"
	"os"
)

// serve is only meaningful in the browser build.
func serve() {
	fmt.Fprintln(os.Stderr, "simwasm: run with -determinism, or build for js/wasm")
	os.Exit(2)
}
