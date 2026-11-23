package httpapi

import (
	"bytes"
	"compress/zlib"
	"context"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestChatSendRateLimitIsSharedByUser(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	s := &Server{deps: app.Dependencies{
		Redis: redisClient,
		Config: config.Config{GlobalSettings: config.GlobalSettings{RateLimit: config.RateLimitSettings{
			ChatSend: config.FixedWindowRateLimitSettings{Max: 2, WindowSeconds: 10},
		}}},
	}}

	for attempt := 1; attempt <= 2; attempt++ {
		limited, _, err := s.hitChatSendRateLimit(context.Background(), 7, "203.0.113.10")
		if err != nil || limited {
			t.Fatalf("attempt %d limited=%v err=%v", attempt, limited, err)
		}
	}
	limited, retryAfter, err := s.hitChatSendRateLimit(context.Background(), 7, "198.51.100.9")
	if err != nil || !limited || retryAfter <= 0 {
		t.Fatalf("same user should remain limited across connections: limited=%v retry=%s err=%v", limited, retryAfter, err)
	}
	limited, _, err = s.hitChatSendRateLimit(context.Background(), 8, "203.0.113.10")
	if err != nil || limited {
		t.Fatalf("independent user should retain a normal allowance: limited=%v err=%v", limited, err)
	}
}

func TestWSRequestIDValidationBoundsRedisKeys(t *testing.T) {
	for _, value := range []string{"ws-123-abc", "2c92240d-49d0-4e7b-a6fa-51c79c7e457e", "client:request_1"} {
		if !isValidWSRequestID(value) {
			t.Fatalf("valid request id rejected: %q", value)
		}
	}
	for _, value := range []string{"", "contains space", "line\nbreak", strings.Repeat("x", wsRequestIDMaxBytes+1)} {
		if isValidWSRequestID(value) {
			t.Fatalf("invalid request id accepted: %q", value)
		}
	}
}

func TestDecodeWSCompressedFrameRejectsInflationAboveLimit(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), int(wsIncomingMessageMaxBytes)+1)
	frame, err := encodeWSCompressedFrame(oversized, zlib.BestCompression)
	if err != nil {
		t.Fatalf("encode oversized frame: %v", err)
	}
	if _, err = decodeWSCompressedFrame(frame); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized inflated frame error=%v", err)
	}

	ordinary := bytes.Repeat([]byte("b"), 1024)
	frame, err = encodeWSCompressedFrame(ordinary, zlib.BestSpeed)
	if err != nil {
		t.Fatalf("encode ordinary frame: %v", err)
	}
	decoded, err := decodeWSCompressedFrame(frame)
	if err != nil || !bytes.Equal(decoded, ordinary) {
		t.Fatalf("ordinary frame decode bytes=%d err=%v", len(decoded), err)
	}
}
