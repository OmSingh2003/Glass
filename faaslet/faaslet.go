package main

import "unsafe"

// --- Host Function Imports ---
// These declarations allow our Wasm module to call functions defined in the host.

//go:wasm-module env
//export set_state
func setState(keyPtr, keyLen, value uint32)

//go:wasm-module env
//export get_state
func getState(keyPtr, keyLen uint32) uint64

// --- Wasm Function Exports ---
// These functions are exported to be called by the host.

//export rate_limit
func rateLimit(clientID, limit, windowSeconds uint64) uint64 {
	// Create a key for this client's rate limit window
	// Format: "rate_limit:clientID:currentWindow"
	currentWindow := getCurrentWindow(windowSeconds)
	key := formatRateLimitKey(clientID, currentWindow)
	keyPtr, keyLen := stringToPtr(key)

	// Get current request count for this client in this window
	current := getState(keyPtr, keyLen)

	// Check if rate limit exceeded
	if current >= limit {
		return 0 // Rate limited
	}

	// Increment the counter
	setState(keyPtr, keyLen, uint32(current+1))
	return limit - current - 1 // Remaining requests
}

//export create_session
func createSession(userID uint64) uint64 {
	// Generate a simple session ID (in real implementation, use proper UUID)
	sessionID := generateSessionID()
	sessionKey := formatSessionKey(sessionID)
	keyPtr, keyLen := stringToPtr(sessionKey)

	// Store session data
	setState(keyPtr, keyLen, uint32(userID))
	return sessionID
}

//export validate_session
func validateSession(sessionID uint64) uint64 {
	sessionKey := formatSessionKey(sessionID)
	keyPtr, keyLen := stringToPtr(sessionKey)
	userID := getState(keyPtr, keyLen)
	return userID // 0 if invalid
}

//export check_feature_flag
func checkFeatureFlag(userID, flagID uint64) uint64 {
	// Check user-specific override
	userKey := formatUserFlagKey(userID, flagID)
	keyPtr, keyLen := stringToPtr(userKey)
	if getState(keyPtr, keyLen) > 0 {
		return 1 // Enabled for user
	}

	// Check global flag
	globalKey := formatGlobalFlagKey(flagID)
	globalPtr, globalLen := stringToPtr(globalKey)
	return getState(globalPtr, globalLen)
}

// --- Helper Functions ---

// stringToPtr is a helper function to pass strings to the host.
func stringToPtr(s string) (uint32, uint32) {
	buf := []byte(s)
	ptr := &buf[0]
	return uint32(uintptr(unsafe.Pointer(ptr))), uint32(len(s))
}

// getCurrentWindow returns the current time window for rate limiting
// This is a simplified implementation - in reality you'd get actual timestamp
func getCurrentWindow(windowSeconds uint64) uint64 {
	// In production, this would use actual timestamp / windowSeconds
	return 1 // Simplified for demo
}

// generateSessionID generates a simple session ID
func generateSessionID() uint64 {
	// Simplified session ID generation
	// In production, use proper UUID or secure random generation
	return 12345 + getCurrentWindow(1) // Simple demo implementation
}

// Helper functions to format different types of keys
func formatRateLimitKey(clientID, window uint64) string {
	return "rate_limit:" + uint64ToString(clientID) + ":" + uint64ToString(window)
}

func formatSessionKey(sessionID uint64) string {
	return "session:" + uint64ToString(sessionID)
}

func formatUserFlagKey(userID, flagID uint64) string {
	return "flag:" + uint64ToString(userID) + ":user:" + uint64ToString(flagID)
}

func formatGlobalFlagKey(flagID uint64) string {
	return "flag:global:" + uint64ToString(flagID)
}

// Simple uint64 to string conversion for demo
func uint64ToString(n uint64) string {
	if n == 0 {
		return "0"
	}

	var result string
	for n > 0 {
		digit := n % 10
		result = string(rune('0'+digit)) + result
		n /= 10
	}
	return result
}

// main is required by TinyGo to compile to Wasm.
func main() {}
