#!/bin/bash

# Glass Performance Benchmarking Script
# Tests various aspects of the Glass platform performance

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Glass Performance Benchmarking Suite${NC}"
echo -e "${BLUE}=====================================${NC}"

# Check if Glass is running
if ! pgrep -f "glass.*-mode=server" > /dev/null; then
    echo -e "${YELLOW}🔧 Starting Glass server...${NC}"
    ./glass -mode=server -port=8080 &
    GLASS_PID=$!
    sleep 3
    
    # Check if Glass started successfully
    if ! curl -s http://localhost:8080/health > /dev/null; then
        echo -e "${RED}❌ Failed to start Glass server${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ Glass server started successfully${NC}"
else
    echo -e "${GREEN}✅ Glass server is already running${NC}"
fi

# Function to run performance tests
run_benchmark() {
    local test_name=$1
    local url=$2
    local concurrent_requests=$3
    local total_requests=$4
    
    echo -e "\n${YELLOW}📊 Running ${test_name}...${NC}"
    echo -e "   Concurrent requests: ${concurrent_requests}"
    echo -e "   Total requests: ${total_requests}"
    
    # Run the benchmark using curl in parallel
    start_time=$(date +%s.%N)
    
    # Create a temporary file for results
    temp_file=$(mktemp)
    
    # Function to make a single request
    make_request() {
        local response_time=$(curl -w "%{time_total}" -s -o /dev/null "$url")
        echo "$response_time" >> "$temp_file"
    }
    
    # Run concurrent requests
    for ((i=1; i<=total_requests; i++)); do
        make_request &
        
        # Limit concurrency
        if (( i % concurrent_requests == 0 )); then
            wait
        fi
    done
    wait
    
    end_time=$(date +%s.%N)
    total_time=$(echo "$end_time - $start_time" | bc)
    
    # Calculate statistics
    if [ -s "$temp_file" ]; then
        avg_response_time=$(awk '{sum+=$1} END {print sum/NR}' "$temp_file")
        min_response_time=$(sort -n "$temp_file" | head -1)
        max_response_time=$(sort -n "$temp_file" | tail -1)
        requests_per_sec=$(echo "scale=2; $total_requests / $total_time" | bc)
        
        echo -e "   ${GREEN}Results:${NC}"
        echo -e "   - Total time: ${total_time}s"
        echo -e "   - Requests per second: ${requests_per_sec}"
        echo -e "   - Average response time: ${avg_response_time}s"
        echo -e "   - Min response time: ${min_response_time}s"
        echo -e "   - Max response time: ${max_response_time}s"
    else
        echo -e "   ${RED}❌ No successful requests${NC}"
    fi
    
    rm -f "$temp_file"
}

# Function to test distributed counters
test_distributed_counters() {
    echo -e "\n${YELLOW}🧮 Testing Distributed Counters...${NC}"
    
    # Test different counter types
    for counter_type in "simple" "partitioned" "crdt" "ratelimit"; do
        echo -e "\n   Testing ${counter_type} counters..."
        
        start_time=$(date +%s.%N)
        
        # Make multiple counter increment requests
        for ((i=1; i<=50; i++)); do
            curl -s "http://localhost:8080/invoke/add?value=1" > /dev/null &
        done
        wait
        
        end_time=$(date +%s.%N)
        duration=$(echo "$end_time - $start_time" | bc)
        
        echo -e "   - ${counter_type} counter test completed in ${duration}s"
    done
}

# Function to test logging performance
test_logging_performance() {
    echo -e "\n${YELLOW}📝 Testing Logging Performance...${NC}"
    
    # Check log file size before and after
    if [ -f "glass.log" ]; then
        initial_size=$(stat -f%z glass.log 2>/dev/null || stat -c%s glass.log 2>/dev/null || echo "0")
    else
        initial_size=0
    fi
    
    # Generate log entries
    start_time=$(date +%s.%N)
    
    for ((i=1; i<=100; i++)); do
        curl -s "http://localhost:8080/invoke/multiply?value=2" > /dev/null &
    done
    wait
    
    end_time=$(date +%s.%N)
    duration=$(echo "$end_time - $start_time" | bc)
    
    if [ -f "glass.log" ]; then
        final_size=$(stat -f%z glass.log 2>/dev/null || stat -c%s glass.log 2>/dev/null || echo "0")
        log_growth=$((final_size - initial_size))
    else
        log_growth=0
    fi
    
    echo -e "   - Log generation test completed in ${duration}s"
    echo -e "   - Log growth: ${log_growth} bytes"
}

# Function to test memory usage
test_memory_usage() {
    echo -e "\n${YELLOW}🧠 Testing Memory Usage...${NC}"
    
    # Get Glass process memory usage
    glass_pid=$(pgrep -f "glass.*-mode=server")
    if [ -n "$glass_pid" ]; then
        memory_usage=$(ps -o rss= -p "$glass_pid" | awk '{print $1}')
        echo -e "   - Glass process memory usage: ${memory_usage} KB"
        
        # Test memory usage under load
        echo -e "   - Testing memory usage under load..."
        initial_memory=$memory_usage
        
        # Generate load
        for ((i=1; i<=200; i++)); do
            curl -s "http://localhost:8080/invoke/add?value=1" > /dev/null &
        done
        wait
        
        sleep 1
        final_memory=$(ps -o rss= -p "$glass_pid" | awk '{print $1}')
        memory_delta=$((final_memory - initial_memory))
        
        echo -e "   - Memory usage after load: ${final_memory} KB"
        echo -e "   - Memory delta: ${memory_delta} KB"
    else
        echo -e "   ${RED}❌ Glass process not found${NC}"
    fi
}

# Run benchmarks
echo -e "\n${BLUE}🏃 Running Performance Tests${NC}"

# Test 1: Basic function invocation
run_benchmark "Basic Function Invocation" "http://localhost:8080/invoke/add?value=5" 10 100

# Test 2: Session management
run_benchmark "Session Management" "http://localhost:8080/invoke/create_session?value=12345" 5 50

# Test 3: Rate limiting
run_benchmark "Rate Limiting" "http://localhost:8080/invoke/rate_limit?value=1" 20 100

# Test 4: Feature flags
run_benchmark "Feature Flags" "http://localhost:8080/invoke/check_feature_flag?value=1" 15 75

# Test distributed counters
test_distributed_counters

# Test logging performance
test_logging_performance

# Test memory usage
test_memory_usage

# Health check
echo -e "\n${YELLOW}🏥 Health Check${NC}"
health_response=$(curl -s http://localhost:8080/health)
echo -e "   Health Status: ${health_response}"

# Metrics
echo -e "\n${YELLOW}📈 Metrics${NC}"
metrics_response=$(curl -s http://localhost:8080/metrics)
echo -e "   Metrics: ${metrics_response}"

echo -e "\n${GREEN}🎉 Benchmarking Complete!${NC}"
echo -e "${BLUE}=====================================${NC}"

# Optional: Stop Glass if we started it
if [ -n "$GLASS_PID" ]; then
    echo -e "\n${YELLOW}🛑 Stopping Glass server...${NC}"
    kill $GLASS_PID
    wait $GLASS_PID 2>/dev/null
    echo -e "${GREEN}✅ Glass server stopped${NC}"
fi
