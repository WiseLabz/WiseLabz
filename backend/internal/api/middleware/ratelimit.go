package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimit returns middleware that throttles requests per key (e.g. client
// IP or user ID) using a token bucket: burst requests immediately, then
// refilling at ratePerSecond. Callers over the limit get 429 with Retry-After.
//
// ponytail: one limiter per key lives for the process lifetime, swept every
// 10 minutes; fine for a single-instance deployment. If this ever runs
// behind multiple backend replicas, move the bucket to a shared store (Redis)
// so limits are enforced per key across instances, not per instance.
func RateLimit(ctx context.Context, ratePerSecond float64, burst int, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	l := &limiterStore{limiters: make(map[string]*visitor), rate: rate.Limit(ratePerSecond), burst: burst}
	go l.sweepLoop(ctx)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			if key == "" || !l.allow(key) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"code":"rate_limited","message":"too many requests, try again later"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// maxLimiters bounds the limiter map so a flood of distinct keys can't grow
// it without limit. Once full (after an eager sweep), new keys are rejected.
const maxLimiters = 100_000

// IPKey normalizes a client IP into a rate-limit key: IPv6 addresses collapse
// to their /64 (one subscriber's whole allocation shares a bucket, so rotating
// addresses within it buys nothing); IPv4 and unparsable values pass through.
func IPKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		if host, _, splitErr := net.SplitHostPort(ip); splitErr == nil {
			addr, err = netip.ParseAddr(host)
		}
		if err != nil {
			return ip
		}
	}
	addr = addr.Unmap()
	if addr.Is6() {
		if p, err := addr.Prefix(64); err == nil {
			return p.String()
		}
	}
	return addr.String()
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type limiterStore struct {
	mu       sync.Mutex
	limiters map[string]*visitor
	rate     rate.Limit
	burst    int
}

func (l *limiterStore) allow(key string) bool {
	l.mu.Lock()
	v, ok := l.limiters[key]
	if !ok {
		if len(l.limiters) >= maxLimiters {
			l.sweepLocked()
			if len(l.limiters) >= maxLimiters {
				l.mu.Unlock()
				return false
			}
		}
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.limiters[key] = v
	}
	v.lastSeen = time.Now()
	l.mu.Unlock()
	return v.limiter.Allow()
}

func (l *limiterStore) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.mu.Lock()
			l.sweepLocked()
			l.mu.Unlock()
		}
	}
}

func (l *limiterStore) sweepLocked() {
	for key, v := range l.limiters {
		if time.Since(v.lastSeen) > 10*time.Minute {
			delete(l.limiters, key)
		}
	}
}
