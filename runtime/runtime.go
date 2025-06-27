package runtime

import (
	"context"
	"fmt"
	"log"
	"os"

	"glass/state"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WasmRuntime encapsulates the wazero runtime and compiled module.
type WasmRuntime struct {
	Runtime        wazero.Runtime
	CompiledModule wazero.CompiledModule
}

// NewRuntime creates and initializes a new WasmRuntime.
func NewRuntime(stateManager *state.Manager) (*WasmRuntime, error) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	// Instantiate WASI, which is required for TinyGo binaries.
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	// Define and export host functions
	_, err := runtime.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(makeSetState(stateManager)).Export("set_state").
		NewFunctionBuilder().WithFunc(makeGetState(stateManager)).Export("get_state").
		Instantiate(ctx)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	// Load and compile the Wasm module
	wasmBytes, err := os.ReadFile("main.wasm")
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to read wasm file: %w", err)
	}

	compiledModule, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		runtime.Close(ctx)
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}

	return &WasmRuntime{
		Runtime:        runtime,
		CompiledModule: compiledModule,
	}, nil
}

// makeSetState is a factory function for the setState host function.
func makeSetState(sm *state.Manager) func(context.Context, api.Module, uint32, uint32, uint32) {
	return func(ctx context.Context, module api.Module, keyPtr, keyLen, value uint32) {
		keyBytes, ok := module.Memory().Read(keyPtr, keyLen)
		if !ok {
			log.Panicf("Memory.Read(%d, %d) out of range", keyPtr, keyLen)
		}
		key := string(keyBytes)

		if err := sm.Set(ctx, key, uint64(value)); err != nil {
			log.Panicf("failed to set state for key '%s': %v", key, err)
		}
		log.Printf("HOST: State '%s' set to %d in Redis", key, value)
	}
}

// makeGetState is a factory function for the getState host function.
func makeGetState(sm *state.Manager) func(context.Context, api.Module, uint32, uint32) uint64 {
	return func(ctx context.Context, module api.Module, keyPtr, keyLen uint32) uint64 {
		keyBytes, ok := module.Memory().Read(keyPtr, keyLen)
		if !ok {
			log.Panicf("Memory.Read(%d, %d) out of range", keyPtr, keyLen)
		}
		key := string(keyBytes)

		val, err := sm.Get(ctx, key)
		if err != nil {
			log.Panicf("failed to get state for key '%s': %v", key, err)
		}
		log.Printf("HOST: Read state '%s' with value %d from Redis", key, val)
		return val
	}
}
