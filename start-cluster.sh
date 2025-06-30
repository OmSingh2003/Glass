#!/bin/bash

# Script to start multiple Glass instances for load balancer testing

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting Glass cluster...${NC}"

# Build the main Glass application
echo -e "${YELLOW}Building Glass application...${NC}"
./build.sh

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to build Glass application${NC}"
    exit 1
fi

# Build the load balancer
echo -e "${YELLOW}Building load balancer...${NC}"
cd loadbalancer
go build -o loadbalancer .
cd ..

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to build load balancer${NC}"
    exit 1
fi

# Kill any existing processes
echo -e "${YELLOW}Stopping any existing Glass instances...${NC}"
pkill -f "glass.*-port"
pkill -f "loadbalancer"

# Wait a moment for processes to stop
sleep 2

# Start Glass instances on different ports
echo -e "${YELLOW}Starting Glass instance 1 on port 9091...${NC}"
./glass -mode=server -port=9091 -node-id=glass-node-1 > glass-node-1.log 2>&1 &
GLASS_PID_1=$!

echo -e "${YELLOW}Starting Glass instance 2 on port 9092...${NC}"
./glass -mode=server -port=9092 -node-id=glass-node-2 > glass-node-2.log 2>&1 &
GLASS_PID_2=$!

echo -e "${YELLOW}Starting Glass instance 3 on port 9093...${NC}"
./glass -mode=server -port=9093 -node-id=glass-node-3 > glass-node-3.log 2>&1 &
GLASS_PID_3=$!

# Wait for Glass instances to start
echo -e "${YELLOW}Waiting for Glass instances to start...${NC}"
sleep 3

# Check if Glass instances are running
for port in 9091 9092 9093; do
    if curl -s "http://localhost:$port/health" > /dev/null; then
        echo -e "${GREEN}Glass instance on port $port is healthy${NC}"
    else
        echo -e "${RED}Glass instance on port $port failed to start${NC}"
    fi
done

# Start the load balancer
echo -e "${YELLOW}Starting load balancer on port 8080...${NC}"
./loadbalancer/loadbalancer -port=8080 -nodes="localhost:9091,localhost:9092,localhost:9093" > loadbalancer.log 2>&1 &
LB_PID=$!

# Wait for load balancer to start
sleep 2

# Check if load balancer is running
if curl -s "http://localhost:8080/lb/health" > /dev/null; then
    echo -e "${GREEN}Load balancer is healthy${NC}"
else
    echo -e "${RED}Load balancer failed to start${NC}"
    exit 1
fi

echo -e "${GREEN}Glass cluster started successfully!${NC}"
echo ""
echo "Services running:"
echo "  - Glass Node 1: http://localhost:9091 (PID: $GLASS_PID_1)"
echo "  - Glass Node 2: http://localhost:9092 (PID: $GLASS_PID_2)"
echo "  - Glass Node 3: http://localhost:9093 (PID: $GLASS_PID_3)"
echo "  - Load Balancer: http://localhost:8080 (PID: $LB_PID)"
echo ""
echo "Endpoints:"
echo "  - Load balancer health: http://localhost:8080/lb/health"
echo "  - Load balancer status: http://localhost:8080/lb/status"
echo "  - Function invocation: http://localhost:8080/invoke/{function_name}"
echo ""
echo "Log files:"
echo "  - Glass Node 1: glass-node-1.log"
echo "  - Glass Node 2: glass-node-2.log"
echo "  - Glass Node 3: glass-node-3.log"
echo "  - Load Balancer: loadbalancer.log"
echo ""
echo -e "${YELLOW}To stop the cluster, run: ./stop-cluster.sh${NC}"
