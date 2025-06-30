package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Node represents a Glass server instance
type Node struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	Healthy  bool      `json:"healthy"`
	LastSeen time.Time `json:"last_seen"`
	Load     int       `json:"load"` // Current number of active requests
}

// VirtualNode represents a virtual node in the hash ring
type VirtualNode struct {
	Hash   uint64
	NodeID string
}

// ConsistentHashRing implements consistent hashing with virtual nodes
type ConsistentHashRing struct {
	mu           sync.RWMutex
	nodes        map[string]*Node
	virtualNodes []VirtualNode
	replicas     int // Number of virtual nodes per physical node
}

// NewConsistentHashRing creates a new consistent hash ring
func NewConsistentHashRing(replicas int) *ConsistentHashRing {
	return &ConsistentHashRing{
		nodes:    make(map[string]*Node),
		replicas: replicas,
	}
}

// hash computes SHA256 hash of the input string
func (chr *ConsistentHashRing) hash(key string) uint64 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(h[:8])
}

// AddNode adds a node to the hash ring
func (chr *ConsistentHashRing) AddNode(node *Node) {
	chr.mu.Lock()
	defer chr.mu.Unlock()

	chr.nodes[node.ID] = node

	// Add virtual nodes
	for i := 0; i < chr.replicas; i++ {
		virtualKey := fmt.Sprintf("%s:%d", node.ID, i)
		hash := chr.hash(virtualKey)
		chr.virtualNodes = append(chr.virtualNodes, VirtualNode{
			Hash:   hash,
			NodeID: node.ID,
		})
	}

	// Sort virtual nodes by hash
	sort.Slice(chr.virtualNodes, func(i, j int) bool {
		return chr.virtualNodes[i].Hash < chr.virtualNodes[j].Hash
	})

	log.Printf("Added node %s (%s) to hash ring", node.ID, node.Address)
}

// RemoveNode removes a node from the hash ring
func (chr *ConsistentHashRing) RemoveNode(nodeID string) {
	chr.mu.Lock()
	defer chr.mu.Unlock()

	delete(chr.nodes, nodeID)

	// Remove virtual nodes
	newVirtualNodes := make([]VirtualNode, 0)
	for _, vn := range chr.virtualNodes {
		if vn.NodeID != nodeID {
			newVirtualNodes = append(newVirtualNodes, vn)
		}
	}
	chr.virtualNodes = newVirtualNodes

	log.Printf("Removed node %s from hash ring", nodeID)
}

// GetNode returns the node responsible for the given key
func (chr *ConsistentHashRing) GetNode(key string) *Node {
	chr.mu.RLock()
	defer chr.mu.RUnlock()

	if len(chr.virtualNodes) == 0 {
		return nil
	}

	hash := chr.hash(key)

	// Find the first virtual node with hash >= key hash
	idx := sort.Search(len(chr.virtualNodes), func(i int) bool {
		return chr.virtualNodes[i].Hash >= hash
	})

	// Wrap around if necessary
	if idx == len(chr.virtualNodes) {
		idx = 0
	}

	// Find a healthy node starting from the selected virtual node
	for i := 0; i < len(chr.virtualNodes); i++ {
		vn := chr.virtualNodes[(idx+i)%len(chr.virtualNodes)]
		if node, exists := chr.nodes[vn.NodeID]; exists && node.Healthy {
			return node
		}
	}

	return nil // No healthy nodes available
}

// GetHealthyNodes returns all healthy nodes
func (chr *ConsistentHashRing) GetHealthyNodes() []*Node {
	chr.mu.RLock()
	defer chr.mu.RUnlock()

	var healthyNodes []*Node
	for _, node := range chr.nodes {
		if node.Healthy {
			healthyNodes = append(healthyNodes, node)
		}
	}
	return healthyNodes
}

// LoadBalancer represents the main load balancer
type LoadBalancer struct {
	hashRing     *ConsistentHashRing
	healthTicker *time.Ticker
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer() *LoadBalancer {
	ctx, cancel := context.WithCancel(context.Background())
	lb := &LoadBalancer{
		hashRing: NewConsistentHashRing(150), // 150 virtual nodes per physical node
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start health checking
	lb.healthTicker = time.NewTicker(10 * time.Second)
	go lb.healthCheckLoop()

	return lb
}

// AddNode adds a node to the load balancer
func (lb *LoadBalancer) AddNode(nodeID, address string) {
	node := &Node{
		ID:       nodeID,
		Address:  address,
		Healthy:  false, // Will be set to true after first health check
		LastSeen: time.Now(),
		Load:     0,
	}
	lb.hashRing.AddNode(node)
}

// extractRoutingKey extracts a routing key from the HTTP request
func (lb *LoadBalancer) extractRoutingKey(r *http.Request) string {
	// Priority order for routing key extraction:
	
	// 1. User ID from headers (for session affinity)
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return "user:" + userID
	}
	
	// 2. Session ID from headers or cookies
	if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
		return "session:" + sessionID
	}
	if cookie, err := r.Cookie("session_id"); err == nil && cookie.Value != "" {
		return "session:" + cookie.Value
	}
	
	// 3. Function name from URL path (for function locality)
	if strings.HasPrefix(r.URL.Path, "/invoke/") {
		functionName := strings.TrimPrefix(r.URL.Path, "/invoke/")
		if functionName != "" {
			return "function:" + functionName
		}
	}
	
	// 4. Client IP as fallback
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	
	return "ip:" + clientIP
}

// ServeHTTP handles incoming HTTP requests
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health check endpoint
	if r.URL.Path == "/lb/health" {
		lb.handleHealthCheck(w, r)
		return
	}

	// Status endpoint
	if r.URL.Path == "/lb/status" {
		lb.handleStatus(w, r)
		return
	}

	// Extract routing key and select node
	routingKey := lb.extractRoutingKey(r)
	node := lb.hashRing.GetNode(routingKey)

	if node == nil {
		http.Error(w, "No healthy backend nodes available", http.StatusServiceUnavailable)
		return
	}

	// Increment load counter
	lb.hashRing.mu.Lock()
	node.Load++
	lb.hashRing.mu.Unlock()

	// Decrement load counter when request completes
	defer func() {
		lb.hashRing.mu.Lock()
		node.Load--
		lb.hashRing.mu.Unlock()
	}()

	// Proxy the request
	targetURL, err := url.Parse("http://" + node.Address)
	if err != nil {
		http.Error(w, "Invalid backend address", http.StatusInternalServerError)
		return
	}

	// Add routing information to headers
	r.Header.Set("X-LB-Node-ID", node.ID)
	r.Header.Set("X-LB-Routing-Key", routingKey)

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	
	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Proxy error for node %s: %v", node.ID, err)
		// Mark node as unhealthy on proxy errors
		lb.hashRing.mu.Lock()
		node.Healthy = false
		lb.hashRing.mu.Unlock()
		
		http.Error(w, "Backend service unavailable", http.StatusBadGateway)
	}

	proxy.ServeHTTP(w, r)
}

// handleHealthCheck returns load balancer health status
func (lb *LoadBalancer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	healthyNodes := lb.hashRing.GetHealthyNodes()
	status := map[string]interface{}{
		"status":             "healthy",
		"healthy_nodes":      len(healthyNodes),
		"total_nodes":        len(lb.hashRing.nodes),
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleStatus returns detailed status information
func (lb *LoadBalancer) handleStatus(w http.ResponseWriter, r *http.Request) {
	lb.hashRing.mu.RLock()
	nodes := make([]*Node, 0, len(lb.hashRing.nodes))
	for _, node := range lb.hashRing.nodes {
		nodes = append(nodes, node)
	}
	lb.hashRing.mu.RUnlock()

	status := map[string]interface{}{
		"nodes":              nodes,
		"virtual_nodes":      len(lb.hashRing.virtualNodes),
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// healthCheckLoop periodically checks the health of all nodes
func (lb *LoadBalancer) healthCheckLoop() {
	for {
		select {
		case <-lb.ctx.Done():
			return
		case <-lb.healthTicker.C:
			lb.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all nodes
func (lb *LoadBalancer) performHealthChecks() {
	lb.hashRing.mu.RLock()
	nodes := make([]*Node, 0, len(lb.hashRing.nodes))
	for _, node := range lb.hashRing.nodes {
		nodes = append(nodes, node)
	}
	lb.hashRing.mu.RUnlock()

	for _, node := range nodes {
		go lb.checkNodeHealth(node)
	}
}

// checkNodeHealth checks the health of a single node
func (lb *LoadBalancer) checkNodeHealth(node *Node) {
	healthURL := fmt.Sprintf("http://%s/health", node.Address)
	
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(healthURL)
	
	lb.hashRing.mu.Lock()
	defer lb.hashRing.mu.Unlock()

	if err != nil || resp.StatusCode != http.StatusOK {
		if node.Healthy {
			log.Printf("Node %s (%s) is now unhealthy: %v", node.ID, node.Address, err)
		}
		node.Healthy = false
		return
	}

	// Read and parse health response
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		node.Healthy = false
		return
	}

	var healthStatus map[string]interface{}
	if err := json.Unmarshal(body, &healthStatus); err != nil {
		node.Healthy = false
		return
	}

	// Check if the response indicates healthy status
	if status, ok := healthStatus["status"].(string); ok && status == "healthy" {
		if !node.Healthy {
			log.Printf("Node %s (%s) is now healthy", node.ID, node.Address)
		}
		node.Healthy = true
		node.LastSeen = time.Now()
	} else {
		node.Healthy = false
	}
}

// Stop gracefully stops the load balancer
func (lb *LoadBalancer) Stop() {
	lb.cancel()
	lb.healthTicker.Stop()
}

func main() {
	var (
		port  = flag.String("port", "8080", "Load balancer port")
		nodes = flag.String("nodes", "localhost:9091,localhost:9092,localhost:9093", "Comma-separated list of backend node addresses")
	)
	flag.Parse()

	// Create load balancer
	lb := NewLoadBalancer()
	defer lb.Stop()

	// Add nodes
	nodeAddresses := strings.Split(*nodes, ",")
	for i, addr := range nodeAddresses {
		nodeID := fmt.Sprintf("glass-node-%d", i+1)
		lb.AddNode(nodeID, strings.TrimSpace(addr))
	}

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + *port,
		Handler: lb,
	}

	log.Printf("Glass Load Balancer starting on port %s", *port)
	log.Printf("Backend nodes: %s", *nodes)
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Load balancer failed to start: %v", err)
	}
}
