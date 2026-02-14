package handlers

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRateLimiterDevicePerSecond(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	rl := &RateLimiter{
		rdb:          rdb,
		devicePerSec: 1,
		devicePerMin: 10,
		slotPerMin:   10,
	}

	ok, err := rl.Allow(context.Background(), "dev-1", 1)
	if err != nil {
		t.Fatalf("Allow #1 unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("Allow #1 should be true")
	}

	ok, err = rl.Allow(context.Background(), "dev-1", 1)
	if err != nil {
		t.Fatalf("Allow #2 unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("Allow #2 should be false due to per-second limit")
	}
}

func TestRateLimiterSlotPerMinute(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	rl := &RateLimiter{
		rdb:          rdb,
		devicePerSec: 100,
		devicePerMin: 100,
		slotPerMin:   1,
	}

	ok, err := rl.Allow(context.Background(), "dev-2", 5)
	if err != nil {
		t.Fatalf("Allow #1 unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("Allow #1 should be true")
	}

	ok, err = rl.Allow(context.Background(), "dev-2", 5)
	if err != nil {
		t.Fatalf("Allow #2 unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("Allow #2 should be false due to slot-per-minute limit")
	}

	ok, err = rl.Allow(context.Background(), "dev-2", 6)
	if err != nil {
		t.Fatalf("Allow #3 unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("Allow #3 should be true for different slot")
	}
}
