package usync

import (
	"testing"
	"time"
)

func TestTicker(t *testing.T) {
	ticker := NewTicker(time.Millisecond, 100*time.Millisecond)

	select {
	case <-ticker.C:
	case <-time.After(5 * time.Millisecond):
		t.Fatalf("Initial took too much time")
	}

	select {
	case <-ticker.C:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("Next took too much time")
	}

	ticker.Stop()

	time.Sleep(50 * time.Millisecond)
}
