package middleware

import (
	"net/http"
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
func RateLimit(ratePerSecond float64, burst int, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	l := &limiterStore{limiters: make(map[string]*visitor), rate: rate.Limit(ratePerSecond), burst: burst}
	go l.sweepLoop()

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
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.limiters[key] = v
	}
	v.lastSeen = time.Now()
	l.mu.Unlock()
	return v.limiter.Allow()
}

func (l *limiterStore) sweepLoop() {
	for range time.Tick(10 * time.Minute) {
		l.mu.Lock()
		for key, v := range l.limiters {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(l.limiters, key)
			}
		}
		l.mu.Unlock()
	}
}
