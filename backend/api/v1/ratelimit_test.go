package v1

import (
	"strconv"
	"testing"
	"time"
)

func TestRateLimiterCapacityAndExpiry(t *testing.T) {
	l := newRateLimiter()
	for i := 0; i < maxRateWindows; i++ {
		if ok, _ := l.allow(strconv.Itoa(i), 1, time.Hour); !ok {
			t.Fatal("early capacity rejection")
		}
	}
	if ok, _ := l.allow("new attacker", 1, time.Hour); ok {
		t.Fatal("capacity overflow")
	}
	if ok, _ := l.allow("0", 1, time.Hour); ok {
		t.Fatal("live window evicted")
	}
	l.windows["0"] = rateWindow{Count: 1, Reset: time.Now().Add(-time.Second)}
	if ok, _ := l.allow("legitimate", 1, time.Hour); !ok {
		t.Fatal("expired window not reclaimed")
	}
	l.windows["old"] = rateWindow{Reset: time.Now().Add(-time.Second)}
	l.lastCleanup = time.Now().Add(-time.Minute)
	l.allow("legitimate", 5, time.Hour)
	if _, exists := l.windows["old"]; exists {
		t.Fatal("periodic cleanup did not run")
	}
}
