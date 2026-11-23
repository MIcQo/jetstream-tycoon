//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/MIcQo/jetstream-tycoon/sim/bridge"
)

// serve registers globalThis.jetstreamSim and blocks forever so the Go runtime
// stays alive for calls from the Worker. A Go panic inside a JS callback would
// stop the runtime, so every call goes through bridge.Guard, which turns errors
// and panics into `{"error": "..."}` responses.
func serve() {
	var session bridge.Session
	api := js.Global().Get("Object").New()
	api.Set("newGame", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return bridge.Guard(func() (string, error) { return session.NewGame(args[0].String()) })
	}))
	api.Set("advance", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return bridge.Guard(func() (string, error) { return session.Advance(args[0].Int()) })
	}))
	api.Set("command", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return bridge.Guard(func() (string, error) { return session.Command(args[0].String()), nil })
	}))
	js.Global().Set("jetstreamSim", api)
	select {}
}
