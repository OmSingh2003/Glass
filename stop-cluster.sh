#!/bin/bash

# Script to stop the Glass cluster

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Stopping Glass cluster...${NC}"

# Kill Glass instances
echo -e "${YELLOW}Stopping Glass instances...${NC}"
pkill -f "glass.*-port"

# Kill load balancer
echo -e "${YELLOW}Stopping load balancer...${NC}"
pkill -f "loadbalancer"

# Wait for processes to stop
sleep 2

# Check if processes are still running
if pgrep -f "glass.*-port" > /dev/null; then
    echo -e "${RED}Some Glass instances are still running${NC}"
    echo "Force killing..."
    pkill -9 -f "glass.*-port"
fi

if pgrep -f "loadbalancer" > /dev/null; then
    echo -e "${RED}Load balancer is still running${NC}"
    echo "Force killing..."
    pkill -9 -f "loadbalancer"
fi

echo -e "${GREEN}Glass cluster stopped${NC}"

# Optionally clean up log files
read -p "Do you want to remove log files? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Removing log files...${NC}"
    rm -f glass-node-*.log loadbalancer.log
    echo -e "${GREEN}Log files removed${NC}"
fi
