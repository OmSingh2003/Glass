#!/bin/bash

echo "🛑 Stopping Glass Platform..."
echo "============================="

# Find and kill Glass processes
GLASS_PIDS=$(lsof -t -i :8080 2>/dev/null)

if [ -z "$GLASS_PIDS" ]; then
    echo "✅ No Glass instance found running on port 8080"
else
    echo "🔧 Found Glass instance(s) on port 8080. Stopping..."
    
    for pid in $GLASS_PIDS; do
        echo "   Killing process $pid..."
        kill $pid 2>/dev/null
        
        # Wait a moment and check if it's still running
        sleep 2
        if kill -0 $pid 2>/dev/null; then
            echo "   Force killing process $pid..."
            kill -9 $pid 2>/dev/null
        fi
    done
    
    # Verify port is free
    if lsof -i :8080 > /dev/null 2>&1; then
        echo "❌ Failed to stop all processes on port 8080"
        echo "   Remaining processes:"
        lsof -i :8080
        exit 1
    else
        echo "✅ Glass platform stopped successfully"
    fi
fi

# Also check for any React development servers on port 3000
REACT_PIDS=$(lsof -t -i :3000 2>/dev/null)

if [ -n "$REACT_PIDS" ]; then
    echo "🌐 Found React development server(s) on port 3000. Stopping..."
    
    for pid in $REACT_PIDS; do
        echo "   Killing process $pid..."
        kill $pid 2>/dev/null
    done
    
    sleep 2
    echo "✅ React development server stopped"
fi

echo ""
echo "🎉 All Glass services stopped!"
echo "   Port 8080: Available"
echo "   Port 3000: Available"
echo ""
echo "You can now run ./start-full-stack.sh to start fresh instances."
