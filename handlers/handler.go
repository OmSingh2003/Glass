// /Glass/handlers/handlers.go
package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"glass/runtime"

	"github.com/tetratelabs/wazero"
)

// InvokeHandler handles requests to execute a Wasm function.
func InvokeHandler(wasmRuntime *runtime.WasmRuntime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract function name from URL, e.g., /invoke/add -> "add"
		functionName := r.URL.Path[len("/invoke/"):]
		if functionName == "" {
			http.Error(w, "Function name not provided in URL", http.StatusBadRequest)
			return
		}

		log.Printf("Received request to invoke function: %s", functionName)

		ctx := context.Background()

		// Instantiate a fresh module for each request.
		module, err := wasmRuntime.Runtime.InstantiateModule(ctx, wasmRuntime.CompiledModule, wazero.NewModuleConfig())
		if err != nil {
			log.Printf("Error instantiating module: %v", err)
			http.Error(w, "Failed to instantiate Wasm module", http.StatusInternalServerError)
			return
		}
		defer module.Close(ctx)

		// Get the function from the module
		wasmFunc := module.ExportedFunction(functionName)
		if wasmFunc == nil {
			http.Error(w, fmt.Sprintf("Function '%s' not found in module", functionName), http.StatusNotFound)
			return
		}

		// --- Input/Output Handling ---
		// This is a simple example that takes a single 'value' query parameter.
		// A more robust solution would handle request bodies (e.g., JSON).
		valueStr := r.URL.Query().Get("value")
		var value uint64
		if valueStr != "" {
			value, err = strconv.ParseUint(valueStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid 'value' parameter", http.StatusBadRequest)
				return
			}
		}

		// Call the Wasm function. This example assumes a function signature like: func(a, b uint64) uint64
		results, err := wasmFunc.Call(ctx, value, 0)
		if err != nil {
			log.Printf("Error executing wasm function '%s': %v", functionName, err)
			http.Error(w, "Error executing Wasm function", http.StatusInternalServerError)
			return
		}

		// Send the result back as the response
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Result from '%s': %d", functionName, results[0])
	}
}
