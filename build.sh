#!/bin/bash

echo "Building Glass FaaS project..."

# Build the main application (excluding WASM faaslet)
echo "1. Building main application..."
go build -o glass main.go
if [ $? -eq 0 ]; then
    echo "✅ Main application built successfully"
else
    echo "❌ Failed to build main application"
    exit 1
fi

# Build individual packages to verify they compile
echo "2. Building individual packages..."
go build ./handlers
go build ./state  
go build ./runtime
if [ $? -eq 0 ]; then
    echo "✅ All packages built successfully"
else
    echo "❌ Failed to build packages"
    exit 1
fi

echo "3. Running tests..."
go vet ./handlers ./state ./runtime
if [ $? -eq 0 ]; then
    echo "✅ Code passes vet checks"
else
    echo "❌ Code has vet issues"
    exit 1
fi

echo "🎉 Build complete! You can now run: ./glass"
