//go:build !(js && wasm)

// Command fibwasm has no native implementation.
//
// The real entry point is main.go, which needs syscall/js and therefore only
// builds for GOOS=js GOARCH=wasm. This stub exists so that `go vet ./...`,
// `go test ./...` and `make ci` keep working on a normal Linux or macOS
// checkout: without a buildable file in the directory, `./...` would report the
// package as unbuildable and CI would go red on a machine that is doing nothing
// wrong.
//
// Run `make wasm` to produce the real artifact.
package main

import "fmt"

func main() {
	fmt.Println("fibwasm requires GOOS=js GOARCH=wasm. Run: make wasm")
}
