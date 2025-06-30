#!/bin/bash

# Script to test the load balancer and consistent hashing

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

LB_URL="http://localhost:8080"
TOTAL_REQUESTS=20

echo -e "${GREEN}Testing Glass Load Balancer with Consistent Hashing${NC}"
echo ""

# Check if load balancer is running
if ! curl -s "$LB_URL/lb/health" > /dev/null; then
    echo -e "${RED}Load balancer is not running on $LB_URL${NC}"
    echo "Please start the cluster first with: ./start-cluster.sh"
    exit 1
fi

# Show load balancer status
echo -e "${BLUE}Load Balancer Status:${NC}"
curl -s "$LB_URL/lb/status" | jq '.' 2>/dev/null || curl -s "$LB_URL/lb/status"
echo ""

# Test 1: Session affinity
echo -e "${BLUE}Test 1: Session Affinity${NC}"
echo "Making 5 requests with the same session ID..."

SESSION_ID="session123"
for i in {1..5}; do
    echo -n "Request $i: "
    RESPONSE=$(curl -s -H "X-Session-ID: $SESSION_ID" "$LB_URL/invoke/add?value=10" -w "Node: %{http_header_x-lb-node-id}")
    echo "$RESPONSE"
done
echo ""

# Test 2: User affinity
echo -e "${BLUE}Test 2: User Affinity${NC}"
echo "Making 5 requests with the same user ID..."

USER_ID="user456"
for i in {1..5}; do
    echo -n "Request $i: "
    RESPONSE=$(curl -s -H "X-User-ID: $USER_ID" "$LB_URL/invoke/multiply?value=5" -w "Node: %{http_header_x-lb-node-id}")
    echo "$RESPONSE"
done
echo ""

# Test 3: Function locality
echo -e "${BLUE}Test 3: Function Locality${NC}"
echo "Making requests to the same function (should go to same node)..."

for i in {1..5}; do
    echo -n "Request $i to /invoke/counter: "
    RESPONSE=$(curl -s "$LB_URL/invoke/counter?value=1" -w "Node: %{http_header_x-lb-node-id}")
    echo "$RESPONSE"
done
echo ""

# Test 4: Distribution across different keys
echo -e "${BLUE}Test 4: Distribution Test${NC}"
echo "Making requests with different routing keys to see distribution..."

# Create an associative array to count requests per node
declare -A node_counts

for i in {1..20}; do
    # Use different user IDs to see distribution
    USER_ID="user$i"
    RESPONSE=$(curl -s -H "X-User-ID: $USER_ID" "$LB_URL/invoke/add?value=$i" -w "%{http_header_x-lb-node-id}")
    
    # Extract node ID from response
    NODE_ID=$(echo "$RESPONSE" | grep -o 'glass-node-[0-9]*' | head -1)
    if [ ! -z "$NODE_ID" ]; then
        ((node_counts[$NODE_ID]++))
    fi
    
    echo -n "."
done

echo ""
echo "Request distribution:"
for node in "${!node_counts[@]}"; do
    echo "  $node: ${node_counts[$node]} requests"
done
echo ""

# Test 5: Health check behavior
echo -e "${BLUE}Test 5: Health Check Status${NC}"
echo "Current health status of all nodes:"
curl -s "$LB_URL/lb/status" | jq '.nodes[] | {id: .id, address: .address, healthy: .healthy, load: .load}' 2>/dev/null || {
    echo "jq not available, showing raw JSON:"
    curl -s "$LB_URL/lb/status"
}
echo ""

# Test 6: Concurrent requests
echo -e "${BLUE}Test 6: Concurrent Load Test${NC}"
echo "Sending 50 concurrent requests..."

# Background function for concurrent requests
send_request() {
    local id=$1
    local user_id="concurrent_user_$id"
    curl -s -H "X-User-ID: $user_id" "$LB_URL/invoke/add?value=$id" -w "Node: %{http_header_x-lb-node-id}" > /tmp/glass_test_$id.out
}

# Send concurrent requests
for i in {1..50}; do
    send_request $i &
done

# Wait for all requests to complete
wait

# Count results
echo "Concurrent request results:"
declare -A concurrent_counts
for i in {1..50}; do
    if [ -f "/tmp/glass_test_$i.out" ]; then
        NODE_ID=$(grep -o 'glass-node-[0-9]*' "/tmp/glass_test_$i.out" | head -1)
        if [ ! -z "$NODE_ID" ]; then
            ((concurrent_counts[$NODE_ID]++))
        fi
        rm -f "/tmp/glass_test_$i.out"
    fi
done

for node in "${!concurrent_counts[@]}"; do
    echo "  $node: ${concurrent_counts[$node]} requests"
done
echo ""

echo -e "${GREEN}Load balancer testing complete!${NC}"
echo ""
echo -e "${YELLOW}Key observations:${NC}"
echo "1. Requests with the same session/user ID should go to the same node"
echo "2. Different keys should be distributed across nodes"
echo "3. Function locality should route same functions to same nodes"
echo "4. The system should handle concurrent requests gracefully"
