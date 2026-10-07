package compliance

import (
	"math"
	"time"
)

// DaysLeft returns whole days until expiry, rounded down even after expiry.
func DaysLeft(expiry, now time.Time) int {
	return int(math.Floor(expiry.Sub(now).Hours() / 24))
}
