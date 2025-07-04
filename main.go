package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

// Update 0.1:
// - Uses token bucket algorithm
// - Uses IP based rate limiting technique
// - Refil rate is defined in tokens per seconds

type RateLimiter struct {
	fillCapacity int64
	tokens       int64
	refillRate   int64
	lastRefilled time.Time
}

func (r *RateLimiter) LimiterCheck() bool {

	// get the current time
	// get the time elapsed between now and last refilled time
	// add the number of tokens required but make sure to overflow them if they exceed the fillCapacity
	// substract one token for the request if not rate limited
	now := time.Now()
	duration := now.Sub(r.lastRefilled).Seconds()
	tokensToAdd := int64(math.Floor(duration * float64(r.refillRate)))

	if tokensToAdd > 0 {

		if r.tokens+tokensToAdd > r.fillCapacity {
			r.tokens = r.fillCapacity
		} else {
			r.tokens += tokensToAdd
		}

		r.lastRefilled = now
	}

	fmt.Printf(">=====RATE LIMIT INFO=====<\nTOKENS=%d\nTOADD=%d\nCAPACITY=%d\nDURATION=%f\n", r.tokens, tokensToAdd, r.fillCapacity, duration)
	if r.tokens >= 1 {
		r.tokens -= 1
		return true
	}

	return false

}

type IPRateLimiterMap struct {
	limiters map[string]*RateLimiter
}

func (ipR *IPRateLimiterMap) GetIPRateLimiter(ipAddr string, cap int64, rr int64) *RateLimiter {
	ipLimiter, isMapped := ipR.limiters[ipAddr]

	if !isMapped {
		ipLimiter = InitRateLimiter(cap, rr)
		return ipLimiter
	}

	return ipLimiter
}

func InitIPRateLimiterMap() *IPRateLimiterMap {
	return &IPRateLimiterMap{limiters: make(map[string]*RateLimiter)}
}
func InitRateLimiter(_cap int64, _rate int64) *RateLimiter {
	return &RateLimiter{
		fillCapacity: _cap,
		refillRate:   _rate,
		tokens:       _cap,
		lastRefilled: time.Now(),
	}
}

// Global variable stores the list of rate limiters by mapping each IP with it's rate limiter object
var IPRMap *IPRateLimiterMap

// Inits the rate limiter with defaults
func InitCatto() {
	IPRMap = InitIPRateLimiterMap()
}

func CattoMiddleware(next http.Handler, capacity int64, refillRate int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIPAddr := strings.Split(r.RemoteAddr, ":")[0]

		if IPRMap == nil {
			fmt.Printf("Cannot access catto. Please init first.")
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		rateLimiter := IPRMap.GetIPRateLimiter(clientIPAddr, capacity, refillRate)

		w.Header().Set("X-Ratelimit-Remaining", fmt.Sprintf("%d", rateLimiter.tokens))
		w.Header().Set("X-Ratelimit-Limit", fmt.Sprintf("%d", rateLimiter.fillCapacity))

		if rateLimiter.LimiterCheck() {
			next.ServeHTTP(w, r)

		} else {
			w.Header().Set("X-Ratelimit-Retry-After", "10 Seconds")
			http.Error(w, "Maximum number of requests reached. Please try again later", http.StatusTooManyRequests)
		}
	})
}

// homeHandler handles requests to the root path "/"
func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the home page!")
	w.Write([]byte("OK"))

}

func main() {
	InitCatto()
	mux := http.NewServeMux()

	mux.Handle("/", CattoMiddleware(http.HandlerFunc(homeHandler), 2, 2))
	log.Print("Listening on :3000...")
	err := http.ListenAndServe(":3000", mux)
	log.Fatal(err)

}
