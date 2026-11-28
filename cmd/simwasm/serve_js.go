//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/MIcQo/jetstream-tycoon/sim/bridge"
	"github.com/MIcQo/jetstream-tycoon/sim/detmath"
)

// serve registers globalThis.jetstreamSim and blocks forever so the Go runtime
// stays alive for calls from the Worker. A Go panic inside a JS callback would
// stop the runtime, so every call goes through bridge.Guard, which turns errors
// and panics into `{"error": "..."}` responses.
func serve() {
	var session bridge.Session
	// Production sim.wasm is built with -tags release, so detmath.Finite clamps
	// NaN/±Inf to 0 and calls this hook; the report reaches Sentry via the UI.
	detmath.NonFiniteHook = session.ReportNonFinite
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
	// save, snapshot and delta return a Uint8Array, or an {"error"} JSON string.
	api.Set("save", bytesFunc(session.Save))
	api.Set("snapshot", bytesFunc(session.Snapshot))
	api.Set("delta", bytesFunc(session.Delta))
	// load takes save bytes (Uint8Array) and returns the view JSON or {"error"}.
	api.Set("load", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return bridge.Guard(func() (string, error) {
			data := make([]byte, args[0].Length())
			js.CopyBytesToGo(data, args[0])
			return session.Load(data)
		})
	}))
	js.Global().Set("jetstreamSim", api)
	select {}
}

// bytesFunc wraps a bytes-returning call: the result is copied into a new
// Uint8Array (which the Worker transfers to the UI without another copy).
func bytesFunc(call func() ([]byte, error)) js.Func {
	return js.FuncOf(func(_ js.Value, _ []js.Value) any {
		data, errJSON := bridge.GuardBytes(call)
		if errJSON != "" {
			return errJSON
		}
		arr := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(arr, data)
		return arr
	})
}
