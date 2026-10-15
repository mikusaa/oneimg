package v1

import (
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	Count int
	Reset time.Time
}

type rateLimiter struct {
	*rateState
}

type rateState struct {
	mu          sync.Mutex
	windows     map[string]rateWindow
	lastCleanup time.Time
}

const maxRateWindows = 10000

func newRateLimiter() *rateLimiter {
	state := &rateState{windows: make(map[string]rateWindow)}
	l := &rateLimiter{state}
	stop := make(chan struct{})
	runtime.AddCleanup(l, func(stop chan struct{}) { close(stop) }, stop)
	go sweepRateWindows(state, stop)
	return l
}

// The worker holds state, not its owner; retired routers do not leak a cleanup goroutine.
func sweepRateWindows(state *rateState, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			state.mu.Lock()
			state.cleanup(now)
			state.mu.Unlock()
		case <-stop:
			return
		}
	}
}

func (state *rateState) cleanup(now time.Time) {
	for identity, window := range state.windows {
		if !window.Reset.After(now) {
			delete(state.windows, identity)
		}
	}
	state.lastCleanup = now
}

func (l *rateLimiter) allow(key string, limit int, duration time.Duration) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastCleanup) >= time.Minute || len(l.windows) >= maxRateWindows {
		l.cleanup(now)
	}
	window, exists := l.windows[key]
	if !exists && len(l.windows) >= maxRateWindows {
		return false, time.Minute
	}
	if !exists || !window.Reset.After(now) {
		l.windows[key] = rateWindow{Count: 1, Reset: now.Add(duration)}
		return true, duration
	}
	if window.Count >= limit {
		return false, time.Until(window.Reset)
	}
	window.Count++
	l.windows[key] = window
	return true, time.Until(window.Reset)
}

func (s *Server) limitByIP(name string, limit int, duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) { s.enforceRateLimit(c, name+":"+c.ClientIP(), limit, duration) }
}

func (s *Server) limitByPrincipal(name string, limit int, duration time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := principalFrom(c)
		if !ok {
			c.Next()
			return
		}
		key := name + ":user:" + itoa(principal.User.ID)
		if principal.IsToken() && principal.Token != nil {
			key = name + ":token:" + strconv.FormatUint(uint64(principal.Token.ID), 10)
		}
		s.enforceRateLimit(c, key, limit, duration)
	}
}

func (s *Server) limitBearer(c *gin.Context) {
	principal, ok := principalFrom(c)
	if !ok || !principal.IsToken() || principal.Token == nil {
		c.Next()
		return
	}
	key := "bearer:" + strconv.FormatUint(uint64(principal.Token.ID), 10)
	s.enforceRateLimit(c, key, 300, time.Minute)
}

func (s *Server) enforceRateLimit(c *gin.Context, key string, limit int, duration time.Duration) {
	allowed, retry := s.limiter.allow(key, limit, duration)
	if !allowed {
		seconds := int(retry.Seconds()) + 1
		c.Header("Retry-After", strconv.Itoa(seconds))
		writeProblem(c, http.StatusTooManyRequests, "rate_limit_exceeded", "请求过于频繁，请稍后重试")
		c.Abort()
		return
	}
	c.Next()
}
