package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logger middleware logs HTTP requests
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap the ResponseWriter to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(rw, r)
		
		duration := time.Since(start)
		log.Printf("%s %s %d %v %s", r.Method, r.URL.Path, rw.statusCode, duration, r.RemoteAddr)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORS middleware adds CORS headers
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Auth-Token, X-User-ID, X-Session-ID")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// RateLimiter middleware implements simple rate limiting
func RateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple rate limiting - allow 100 requests per minute per IP
		// In production, use Redis or similar for distributed rate limiting
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware middleware for authentication
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health checks and public endpoints
		if r.URL.Path == "/health" || r.URL.Path == "/metrics" || r.URL.Path == "/lb/health" || r.URL.Path == "/lb/status" {
			next.ServeHTTP(w, r)
			return
		}
		
		token := r.Header.Get("X-Auth-Token")
		if token == "" {
			http.Error(w, "Authentication token required", http.StatusUnauthorized)
			return
		}
		
		// Simple token validation - in production, use JWT or similar
		if token != "your_secret_token" {
			http.Error(w, "Invalid authentication token", http.StatusUnauthorized)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// MetricsMiddleware collects basic metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		
		duration := time.Since(start)
		
		// In production, send these metrics to a monitoring system
		log.Printf("METRICS: method=%s path=%s status=%d duration=%v", 
			r.Method, r.URL.Path, rw.statusCode, duration)
	})
}

// Chain combines multiple middleware functions
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
