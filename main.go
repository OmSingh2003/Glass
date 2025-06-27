package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"glass/runtime"
	"glass/state"

	"github.com/tetratelabs/wazero"
)

func main() {
	ctx := context.Background()

	// Initialize Redis-based state manager
	stateManager, err := state.NewManager()
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()

	// Initialize WASM runtime with state manager
	wasmRuntime, err := runtime.NewRuntime(stateManager)
	if err != nil {
		log.Fatalf("Failed to create WASM runtime: %v", err)
	}

	// Set initial state in Redis only if it doesn't exist
	currentValue, err := stateManager.Get(ctx, "counter")
	if err != nil {
		log.Printf("Counter doesn't exist, setting initial value to 100")
		if err := stateManager.Set(ctx, "counter", 100); err != nil {
			log.Fatalf("Failed to set initial state: %v", err)
		}
	} else {
		log.Printf("Counter exists with value %d, continuing from there", currentValue)
	}

	fmt.Println("--- Running 3 Faaslets Concurrently ---")
	// Spawning multiple Faaslets
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(instanceNum int) {
			defer wg.Done()

			// Instantiate a new module for each "Faaslet". This is like a cold start.
			module, err := wasmRuntime.Runtime.InstantiateModule(ctx, wasmRuntime.CompiledModule, 
				wazero.NewModuleConfig().WithName(fmt.Sprintf("faaslet-%d", instanceNum)))
			if err != nil {
				log.Printf("Failed to instantiate module %d: %v", instanceNum, err)
				return
			}
			defer module.Close(ctx)

			addFn := module.ExportedFunction("add") // calls the add function defined in wasm
			if addFn == nil {
				log.Printf("Function 'add' not found in module %d", instanceNum)
				return
			}

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
				log.Printf("Error in instance %d: %v", instanceNum, err)
				return
			}
			fmt.Printf("Instance %d Result: %d\n", instanceNum, results[0])
		}(i)
	}

	wg.Wait()

	// Get final state from Redis
	finalCounter, err := stateManager.Get(ctx, "counter")
	if err != nil {
		log.Printf("Failed to get final counter: %v", err)
		return
	}

	fmt.Printf("\n--- Final State of 'counter': %d ---\n", finalCounter)
}
