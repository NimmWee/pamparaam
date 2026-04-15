package testsupport

import (
	"testing"
	"time"
)

func Eventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("condition was not satisfied within %s", timeout)
}
