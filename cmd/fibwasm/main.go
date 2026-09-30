//go:build js && wasm

// Command fibwasm exposes internal/wasmapi to the browser.
//
// It is deliberately a one-function surface. wasm_exec.js is a single-threaded,
// message-free runtime: there is no worker, no shared memory, and no way to
// hand a Go value back as anything but a string. So exactly one function is
// registered -- call(method, argsJSON) -> replyJSON -- and all the shaping
// lives on the Go side in internal/wasmapi, which is ordinary Go and is unit
// tested natively under `go test ./internal/wasmapi/`.
//
// The client is expected to:
//
//  1. instantiate the module and start it with wasm_exec.js,
//  2. wait for the global fibReady callback, which is the readiness handshake,
//  3. then call globalThis.fibtransponder.call(...) as often as it likes.
//
// The promise returned by go.run() never settles. That is not a leak: main
// must not return, because the moment the Go runtime exits every later call
// panics. wasm_exec.js surfaces a main-return as a rejected promise, so leaving
// it pending is the correct steady state. Do not "fix" it by removing the
// select below.
package main

import (
	"syscall/js"

	"github.com/shaoyanji/fibtransponder/internal/wasmapi"
)

func main() {
	js.Global().Set("fibtransponder", js.ValueOf(map[string]any{
		"call": js.FuncOf(call),
	}))

	// Explicit handshake. The client installs fibReady before calling go.run,
	// so by the time this fires the dispatcher is already in place. Resolving
	// on a poll loop instead would be a race the other way.
	js.Global().Call("fibReady")
	select {}
}

// call is the whole API surface: a method name and JSON arguments in, a JSON
// envelope out. internal/wasmapi.Call is total -- it converts every failure
// into a coded envelope rather than a panic -- so there is no error path here.
func call(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return string(wasmapi.Call("", nil))
	}
	method := args[0].String()
	raw := args[1].String()
	return string(wasmapi.Call(method, []byte(raw)))
}
