package main

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
)

// Node represents a server node in the cluster.
type Node struct {
	ID   string
	Addr string
}

// ConsistentHashLoadBalancer uses consistent hashing to distribute traffic.
type ConsistentHashLoadBalancer struct {
	mu       sync.RWMutex
	nodes    []Node
	hashRing map[uint32]string // hash ring: hash -> node ID
	sorted   []uint32          // sorted list of hashes
}

// NewConsistentHashLoadBalancer creates a new load balancer.
func NewConsistentHashLoadBalancer(nodes []Node) *ConsistentHashLoadBalancer {
	lb := &ConsistentHashLoadBalancer{
		nodes:    nodes,
		hashRing: make(map[uint32]string),
	}
	lb.generateHashRing()
	return lb
}

// generateHashRing generates a consistent hash ring.
func (lb *ConsistentHashLoadBalancer) generateHashRing() {
	for _, node := range lb.nodes {
		hash := lb.hashID(node.ID)
		lb.hashRing[hash] = node.Addr
		lb.sorted = append(lb.sorted, hash)
	}
	// Sort hashes in ascending order
	sort.Slice(lb.sorted, func(i, j int) bool { return lb.sorted[i] < lb.sorted[j] })
}

// hashID returns the FNV-1a hash of a string ID.
func (lb *ConsistentHashLoadBalancer) hashID(id string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(id))
	return h.Sum32()
}

// GetNode selects a node based on consistent hashing of the key.
func (lb *ConsistentHashLoadBalancer) GetNode(key string) string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	hash := lb.hashID(key)
	// Find the closest hash that is greater than or equal to the key's hash
	for _, h := range lb.sorted {
		if hash <= h {
			return lb.hashRing[h]
		}
	}
	// If not found, wrap around to the first node
	return lb.hashRing[lb.sorted[0]]
}

// ServeHTTP handles HTTP requests by proxying to a selected node.
func (lb *ConsistentHashLoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nodeAddr := lb.GetNode(r.URL.Path)

	targetURL, err := url.Parse("http://" + nodeAddr)
	if err != nil {
		http.Error(w, "Bad node address", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

func main() {
	nodes := []Node{
		{ID: "node1", Addr: "localhost:9091"},
		{ID: "node2", Addr: "localhost:9092"},
		{ID: "node3", Addr: "localhost:9093"},
	}

	lb := NewConsistentHashLoadBalancer(nodes)

	fmt.Println("Load balancer started on :8080")
	http.ListenAndServe(":8080", lb)
}

