package main

import "unsafe"

// Import the host functions we defined in main
//
//go:wasm-module env
//export set_state
func setState(keyPtr, keyLen, value uint32)

//go:wasm-module env
//export get_state
func getState(keyPtr, keyLen uint32) uint64

// main is required for TinyGo to compile.
func main() {}

//export add
func add(a, b uint64) uint64 {
	// Get the current value of "counter" from the host
	counterKey := "counter"
	counterPtr, counterLen := stringToPtr(counterKey)
	currentCounter := getState(counterPtr, counterLen)

	// Calculate the new value
	newValue := currentCounter + a + b

	// Set the new value of "counter" on the host
	setState(counterPtr, counterLen, uint32(newValue))

	return newValue
}

// Helper function to pass strings to the host
func stringToPtr(s string) (uint32, uint32) {
	// This is a bit of a hack to get a pointer to the string data
	// In a real application, we would use a more robust method
	// for memory management.
	buf := []byte(s)
	ptr := &buf[0]
	return uint32(uintptr(unsafe.Pointer(ptr))), uint32(len(s))
}
