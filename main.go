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

	// Initialize some demo feature flags and settings
	if err := stateManager.Set(ctx, "flag:global:1", 1); err != nil {
		log.Printf("Failed to set global feature flag: %v", err)
	}
	log.Printf("Initialized Glass with rate limiting, session management, and feature flags")

	fmt.Println("--- Testing Rate Limiter with Multiple Clients ---")
	// Spawning multiple Faaslets to test rate limiting
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Instantiate a new module for each client
			module, err := wasmRuntime.Runtime.InstantiateModule(ctx, wasmRuntime.CompiledModule, 
				wazero.NewModuleConfig().WithName(fmt.Sprintf("client-%d", clientID)))
			if err != nil {
				log.Printf("Failed to instantiate module for client %d: %v", clientID, err)
				return
			}
			defer module.Close(ctx)

			// Test rate limiting
			rateLimitFn := module.ExportedFunction("rate_limit")
			if rateLimitFn == nil {
				log.Printf("Function 'rate_limit' not found for client %d", clientID)
				return
			}

			// Each client tries to make 3 requests with a limit of 2 per window
			for req := 1; req <= 3; req++ {
				results, err := rateLimitFn.Call(ctx, uint64(clientID), 2, 60) // clientID, limit=2, window=60s
				if err != nil {
					if err.Error() == "module closed with exit_code(0)" {
						continue
					}
					log.Printf("Error in client %d request %d: %v", clientID, req, err)
					continue
				}
				remaining := results[0]
				if remaining == 0 {
					fmt.Printf("Client %d Request %d: RATE LIMITED\n", clientID, req)
				} else {
					fmt.Printf("Client %d Request %d: SUCCESS (remaining: %d)\n", clientID, req, remaining)
				}
			}
		}(i)
	}

	wg.Wait()

	fmt.Println("\n--- Testing Session Management ---")
	// Test session management
	module, err := wasmRuntime.Runtime.InstantiateModule(ctx, wasmRuntime.CompiledModule, wazero.NewModuleConfig())
	if err != nil {
		log.Printf("Failed to instantiate module for session test: %v", err)
		return
	}
	defer module.Close(ctx)

	// Test session creation
	createSessionFn := module.ExportedFunction("create_session")
	if createSessionFn != nil {
		results, err := createSessionFn.Call(ctx, 12345) // userID = 12345
		if err == nil && len(results) > 0 {
			sessionID := results[0]
			fmt.Printf("Created session: %d for user: 12345\n", sessionID)

			// Test session validation
			validateSessionFn := module.ExportedFunction("validate_session")
			if validateSessionFn != nil {
				results, err := validateSessionFn.Call(ctx, sessionID)
				if err == nil && len(results) > 0 {
					userID := results[0]
					fmt.Printf("Session %d validation result - UserID: %d\n", sessionID, userID)
				}
			}
		}
	}

	// Test feature flags
	checkFeatureFn := module.ExportedFunction("check_feature_flag")
	if checkFeatureFn != nil {
		results, err := checkFeatureFn.Call(ctx, 12345, 1) // userID=12345, flagID=1
		if err == nil && len(results) > 0 {
			flagEnabled := results[0]
			fmt.Printf("Feature flag 1 for user 12345: %s\n", map[uint64]string{0: "DISABLED", 1: "ENABLED"}[flagEnabled])
		}
	}

	fmt.Println("\n--- Glass Platform Demo Complete ---")
}
