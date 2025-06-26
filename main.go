package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// variable for simple in-memory state store
var (
	state = make(map[string]uint64) // taken 64 - bit values for robustness and all
	mutex = &sync.Mutex{}           // protect concurrent state map
)

// setState allows the WASM module to write to our shared state
// well basically function will be called from WASM moudule to read a value from Go host's shared state
func setState(ctx context.Context, module api.Module, keyPtr, keyLen, value uint32) {
	// func takes parameters : ctx for cancellation and timeout :general Go stuff
	// module : this represents calling WASM module from wazero runtime
	// keyPtr and KeyLen : a pointer to WASM module's memory where the key string starts and how long it is
	// value is the actual data we want to store in shared state
	// and here's thing WASM typically uses 32 - bit values so i will have to convert it to 64  in the function ahead
	keyBytes, ok := module.Memory().Read(keyPtr, keyLen) // This tries to read a slice of bytes from WASM module's linear memory
	// keyptr is starting address and key len is how mnay bytes to read
	if !ok {
		log.Panicf("Memory.Read(%d, %d) out of range", keyPtr, keyLen) // panic on failure , maybe passed an invalid pointer or something
	}
	key := string(keyBytes) //  coversion to string

	// well go maps are not thread safe so we are
	// Gonna lock the state from anyother goroutine from accessing the map while we are modifying it
	mutex.Lock()
	state[key] = uint64(value) // updating the map and yeah converting to unit64 becasue WASM gives values in 32 - bits
	mutex.Unlock()             // releasing the lock , thinking of maybe using defer

	fmt.Printf("HOST: State '%s' set to %d\n", key, value)
}

// getState allows the WASM module to read a value from the shared state map
// It works similarly to setState, but instead of writing, it performs a read
// It first reads a string key from the WASM module's memory using the provided pointer and length
// If the memory read is valid, it safely looks up the value associated with that key in the Go map
// The value is then returned to the WASM module
func getState(ctx context.Context, module api.Module, keyPtr, keyLen uint32) uint64 {
	keyBytes, ok := module.Memory().Read(keyPtr, keyLen)
	if !ok {
		log.Panicf("Memory.Read(%d, %d) out of range", keyPtr, keyLen)
	}
	key := string(keyBytes)

	mutex.Lock()
	value := state[key]
	mutex.Unlock()

	fmt.Printf("HOST: Read state '%s' with value %d\n", key, value)
	return value
}

func main() {
	ctx := context.Background()       // control timeout and cancellation
	runtime := wazero.NewRuntime(ctx) // creating an WASM runtime instance
	defer runtime.Close(ctx)          // defer right now to ensure everything is cleaned up at the end

	// Expose the host functions to the WASM runtime under the "env" module
	_, err := runtime.NewHostModuleBuilder("env"). // creates a host module name "env"(WASM module typically imports from "env")
		// exporting go functions to be callable inside WASM module
		// so the WASM could now do :
		// import "env"
		// func set_state(ptr, len, value)
		NewFunctionBuilder().WithFunc(setState).Export("set_state").
		NewFunctionBuilder().WithFunc(getState).Export("get_state").
		Instantiate(ctx)
	if err != nil {
		log.Panic(err)
	}
	// Instantiate wasi
	// wasm module compiled throught tinygo
	// This registers WASI functions (like fd_write) to make the module run correctly
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	wasmBytes, err := os.ReadFile("main.wasm") // Read the file from the disk
	if err != nil {
		log.Panic(err)
	}

	// Compile the module once.
	compiledModule, err := runtime.CompileModule(ctx, wasmBytes) // Compile it once so it can Instantiated multiple times
	if err != nil {
		log.Panic(err)
	}

	// Set an initial state
	state["counter"] = 100 // right now manually setting the value
	// this value will be shared among instances

	fmt.Println("--- Running 3 Faaslets Concurrently ---")
	// Spawning multiple Faaslets
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Instantiate a new module for each "Faaslet". This is like a cold start.
			module, err := runtime.InstantiateModule(ctx, compiledModule, wazero.NewModuleConfig().WithName(fmt.Sprintf("faaslet-%d", instanceNum)))
			if err != nil {
				log.Panic(err)
			}
			defer module.Close(ctx)

			addFn := module.ExportedFunction("add") // calls the add function defined in wasm

			// Each instance adds a different value.
			addValue := uint64(instanceNum * 10)
			results, err := addFn.Call(ctx, addValue, 0)
			if err != nil {
				// Check if this is a normal exit (exit code 0)
				if err.Error() == "module closed with exit_code(0)" {
					// This is a successful completion, not an error
					fmt.Printf("Instance %d completed successfully\n", instanceNum)
					return
				}
				log.Panic(err)
			}
			fmt.Printf("Instance %d Result: %d\n", instanceNum, results[0])
		}(i)
	}

	wg.Wait()

	fmt.Printf("\n--- Final State of 'counter': %d ---\n", state["counter"]) // printing shaed counters final value
}
