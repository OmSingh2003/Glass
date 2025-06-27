#!/bin/bash

echo "Building WASM faaslet..."

# Check if TinyGo is installed
if ! command -v tinygo &> /dev/null; then
    echo "TinyGo is not installed. Please install it first:"
    echo "brew install tinygo"
    exit 1
fi

# Build the WASM module
cd faaslet
tinygo build -o ../main.wasm -target wasi .
cd ..

echo "WASM module built successfully: main.wasm"

# Verify the file was created
if [ -f "main.wasm" ]; then
    echo "File size: $(ls -lh main.wasm | awk '{print $5}')"
else
    echo "Error: WASM file was not created"
    exit 1
fi
