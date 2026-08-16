package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	ratePerSecond   = 0.5  // 30 requests per minute per IP
	rateBurst       = 30.0 // allow an initial burst of 30
	rateIdleTimeout = 5 * time.Minute
	rateCleanupTick = time.Minute
)

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rateClient
	rate    float64
	burst   float64
}

type rateClient struct {
	tokens   float64
	lastSeen time.Time
}

func newRateLimiter(rate float64, burst float64) *rateLimiter {
	limiter := &rateLimiter{
		clients: make(map[string]*rateClient),
		rate:    rate,
		burst:   burst,
	}

	go func() {
		ticker := time.NewTicker(rateCleanupTick)
		for range ticker.C {
			limiter.cleanup()
		}
	}()

	return limiter
}

func (l *rateLimiter) allow(ip string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	client, ok := l.clients[ip]
	if !ok {
		client = &rateClient{tokens: l.burst}
		l.clients[ip] = client
	}

	elapsed := now.Sub(client.lastSeen).Seconds()
	client.tokens += elapsed * l.rate
	if client.tokens > l.burst {
		client.tokens = l.burst
	}
	client.lastSeen = now

	if client.tokens < 1 {
		return false
	}

	client.tokens--
	return true
}

func (l *rateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for ip, client := range l.clients {
		if time.Since(client.lastSeen) > rateIdleTimeout {
			delete(l.clients, ip)
		}
	}
}

var defaultLimiter = newRateLimiter(ratePerSecond, rateBurst)

func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !defaultLimiter.allow(clientIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}

	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
