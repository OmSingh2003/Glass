# Glass
Lightweight, shared-memory FaaS with WebAssembly in Go

## Features

- **WebAssembly Runtime**: Uses wazero for fast WASM execution
- **Shared State**: In-memory state management with Redis backend support
- **Concurrent Faaslets**: Multiple WASM instances running concurrently
- **HTTP API**: RESTful endpoints for function execution and state management
- **Modular Architecture**: Clean separation of concerns with separate packages

## Project Structure

```
.
├── main.go                 # Main application with embedded runtime
├── faaslet/               # WASM source code (TinyGo)
│   └── faaslet.go         # Exported functions for WASM
├── handlers/              # HTTP request handlers
│   └── handler.go         # Function invocation endpoints
├── runtime/               # WASM runtime management
│   └── runtime.go         # Wazero runtime wrapper
├── state/                 # State management
│   └── state.go           # Redis-backed state store
├── build.sh               # Build script for Go packages
├── build-wasm.sh          # TinyGo WASM compilation script
└── main.wasm              # Compiled WASM module
```

## Building

### Prerequisites

- Go 1.24.4+
- TinyGo (for WASM compilation)
- Redis (optional, for persistent state)

### Build Instructions

1. **Build the main application:**
   ```bash
   ./build.sh
   ```

2. **Rebuild WASM module (if needed):**
   ```bash
   ./build-wasm.sh
   ```

3. **Run the application:**
   ```bash
   ./glass
   ```

## Usage

The application demonstrates concurrent WASM execution with shared state:

```
--- Running 3 Faaslets Concurrently ---
HOST: Read state 'counter' with value 100
HOST: State 'counter' set to 120
Instance 2 completed successfully
...
--- Final State of 'counter': 160 ---
```

## Demo

![Glass Demo](images/demo.png)
