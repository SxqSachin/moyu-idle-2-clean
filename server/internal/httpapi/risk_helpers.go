package httpapi

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/risk"
)

func (s *Server) recordRiskSignal(c *fiber.Ctx, userID int64, eventType string, deviceProfile risk.DeviceProfile, passwordRaw string, extra map[string]any) {
	if s.deps.Risk == nil || userID <= 0 {
		return
	}
	input := risk.SignalInput{
		UserID:         userID,
		EventType:      eventType,
		IP:             c.IP(),
		UserAgent:      c.Get(fiber.HeaderUserAgent),
		AcceptLanguage: strings.TrimSpace(c.Get(fiber.HeaderAcceptLanguage)),
		PasswordRaw:    passwordRaw,
		DeviceProfile:  deviceProfile,
		Extra:          extra,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.deps.Risk.RecordSignal(ctx, input)
		_ = s.deps.Risk.RecomputeImpactedCluster(ctx, userID)
	}()
}
