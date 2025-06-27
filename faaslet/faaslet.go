package main

import "unsafe"

// --- Host Function Imports ---
// These declarations allow our Wasm module to call functions defined in the host.

//go:wasm-module env
//export set_state
func setState(keyPtr, keyLen, value uint32)

//go:wasm-module env
//export get_state
func getState(keyPtr, keyLen uint32) uint64

// --- Wasm Function Exports ---
// These functions are exported to be called by the host.

//export add
func add(a, b uint64) uint64 {
	counterKey := "counter"
	counterPtr, counterLen := stringToPtr(counterKey)

	// Get the current value of "counter" from the host's state store
	currentCounter := getState(counterPtr, counterLen)

	// Calculate the new value
	newValue := currentCounter + a + b

	// Set the new value of "counter" on the host
	setState(counterPtr, counterLen, uint32(newValue))

	return newValue
}

// --- Helper Functions ---

// stringToPtr is a helper function to pass strings to the host.
func stringToPtr(s string) (uint32, uint32) {
	buf := []byte(s)
	ptr := &buf[0]
	return uint32(uintptr(unsafe.Pointer(ptr))), uint32(len(s))
}

// main is required by TinyGo to compile to Wasm.
func main() {}
