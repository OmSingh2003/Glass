#!/bin/bash

# Glass Full Stack Startup Script
# Starts both the Glass WebAssembly FaaS platform and the React web dashboard

echo "🔥 Starting Glass Full Stack Platform..."
echo "======================================"

# Check if Glass binary exists
if [ ! -f "./glass" ]; then
    echo "❌ Glass binary not found. Building..."
    ./build.sh
    if [ $? -ne 0 ]; then
        echo "❌ Failed to build Glass platform"
        exit 1
    fi
fi

# Check if node_modules exists for the frontend
if [ ! -d "./web-frontend/glass-dashboard/node_modules" ]; then
    echo "📦 Installing frontend dependencies..."
    cd web-frontend/glass-dashboard
    npm install
    if [ $? -ne 0 ]; then
        echo "❌ Failed to install frontend dependencies"
        exit 1
    fi
    cd ../..
fi

echo ""
echo "🚀 Starting services..."
echo ""

# Function to cleanup background processes
cleanup() {
    echo ""
    echo "🛑 Shutting down services..."
    
    # Kill React development server
    if [ -n "$REACT_PID" ]; then
        kill $REACT_PID 2>/dev/null
    fi
    
    # Only kill Glass if we started it
    if [ "$GLASS_RUNNING" = "false" ] && [ -n "$GLASS_PID" ]; then
        kill $GLASS_PID 2>/dev/null
        echo "🔧 Stopped Glass platform"
    fi
    
    # Kill any remaining background jobs
    jobs -p | xargs -r kill 2>/dev/null
    
    echo "✅ Services stopped"
    exit 0
}

# Set trap to cleanup on script exit
trap cleanup SIGINT SIGTERM

# Check if port 8080 is already in use
if lsof -i :8080 > /dev/null 2>&1; then
    echo "⚠️  Port 8080 is already in use. Checking if it's Glass..."
    
    # Check if it's already a Glass instance
    if curl -s http://localhost:8080/health | grep -q "healthy"; then
        echo "✅ Glass platform is already running on port 8080!"
        GLASS_RUNNING=true
    else
        echo "❌ Port 8080 is occupied by another service."
        echo "   Please stop the other service or use a different port."
        echo "   "
        echo "   To kill the existing process:"
        lsof -i :8080 | grep LISTEN
        echo "   "
        echo "   Then run this script again."
        exit 1
    fi
else
    # Start Glass platform in background
    echo "🔧 Starting Glass platform on http://localhost:8080..."
    ./glass -mode=server -port=8080 &
    GLASS_PID=$!
    
    # Wait a moment for Glass to start
    sleep 3
    
    # Check if Glass started successfully
    if ! curl -s http://localhost:8080/health > /dev/null; then
        echo "❌ Failed to start Glass platform"
        kill $GLASS_PID 2>/dev/null
        exit 1
    fi
    
    echo "✅ Glass platform started successfully!"
    GLASS_RUNNING=false
fi

# Start React development server in background
echo "🌐 Starting React dashboard on http://localhost:3000..."
cd web-frontend/glass-dashboard
npm start &
REACT_PID=$!
cd ../..

echo ""
echo "🎉 Glass Full Stack Platform is now running!"
echo "=========================================="
echo ""
echo "🔧 Glass API Server:    http://localhost:8080"
echo "🌐 Glass Dashboard:     http://localhost:3000"
echo ""
echo "📊 Available endpoints:"
echo "   • Health:             http://localhost:8080/health"
echo "   • Function Invoke:    http://localhost:8080/invoke/{function}?value={value}"
echo "   • Metrics:            http://localhost:8080/metrics"
echo ""
echo "🧪 Example function calls:"
echo "   curl \"http://localhost:8080/invoke/add?value=10\""
echo "   curl \"http://localhost:8080/invoke/multiply?value=2\""
echo "   curl \"http://localhost:8080/invoke/get_counter\""
echo ""
echo "💡 Open http://localhost:3000 in your browser to use the Glass Dashboard"
echo ""
echo "Press Ctrl+C to stop all services..."

# Wait for both processes
wait
