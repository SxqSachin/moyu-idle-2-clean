package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/reqmetrics"
)

const (
	slowLoginThreshold           = 800 * time.Millisecond
	slowAPIRequestThreshold      = 1000 * time.Millisecond
	wsPublishSlowThreshold       = 200 * time.Millisecond
	wsDeliverySlowThreshold      = 250 * time.Millisecond
	wsPressureSummaryInterval    = time.Minute
	readinessPressureWindow      = 2 * time.Minute
	readinessWSSlowConsumerLimit = 3
)

const (
	loginStageCaptchaOK          = "captcha_ok"
	loginStageLoginGuardChecked  = "login_guard_checked"
	loginStageAuthStateLoaded    = "auth_state_loaded"
	loginStagePasswordVerified   = "password_verified"
	loginStageUserSettingsLoaded = "user_settings_loaded"
	loginStageIPTrustChecked     = "ip_trust_checked"
	loginStageAttributesEnsured  = "attributes_ensured"
	loginStageStatsRecomputed    = "stats_recomputed"
	loginStagePresenceLoaded     = "presence_loaded"
	loginStageSessionReady       = "session_rotated_or_reused"
	loginStageTokenSigned        = "token_signed"
	loginStageIPLoginRecorded    = "ip_login_recorded"
)

type loginRequestLogState struct {
	startedAt        time.Time
	path             string
	clientIPHash     string
	usernameHash     string
	captchaAccepted  bool
	ipVerifyRequired bool
	sessionRotated   bool
	userID           int64
	finalStatus      int
	finalCode        string
	stages           map[string]float64
}

type wsPressureCounters struct {
	TargetedAttempts        int64
	TargetedDelivered       int64
	TargetedSkipped         int64
	BroadcastAttempts       int64
	BroadcastDelivered      int64
	BroadcastSkipped        int64
	BattleSkippedSubscribed int64
	QueueFullCloses         int64
	SlowConsumerEvents      int64
}

var (
	readinessStoreHealth = func(s *Server, ctx context.Context) error { return s.deps.Store.Health(ctx) }
	readinessAuthHealth  = func(s *Server, ctx context.Context) error { return s.deps.AuthGate.Health(ctx) }
	readinessRedisPing   = func(s *Server, ctx context.Context) error { return s.deps.Redis.Ping(ctx).Err() }
)

func readinessContext(s *Server) (context.Context, context.CancelFunc) {
	timeout := 800 * time.Millisecond
	if s != nil && s.deps.Config.AuthDBTimeoutMS > 0 {
		timeout = time.Duration(s.deps.Config.AuthDBTimeoutMS) * time.Millisecond
	}
	return context.WithTimeout(context.Background(), timeout)
}

func newLoginRequestLogState(path string, ip string, username string) *loginRequestLogState {
	return &loginRequestLogState{
		startedAt:    time.Now(),
		path:         strings.TrimSpace(path),
		clientIPHash: shortHash(ip),
		usernameHash: shortHash(strings.ToLower(strings.TrimSpace(username))),
		stages:       make(map[string]float64, 12),
	}
}

func (s *loginRequestLogState) markStage(stage string) {
	if s == nil || strings.TrimSpace(stage) == "" {
		return
	}
	s.stages[stage] = durationMS(time.Since(s.startedAt))
}

func (s *loginRequestLogState) shouldLog() bool {
	if s == nil {
		return false
	}
	elapsed := time.Since(s.startedAt)
	if elapsed >= slowLoginThreshold {
		return true
	}
	if s.finalStatus >= 500 {
		return true
	}
	if s.finalCode == codeLoginIPVerifyRequired {
		return true
	}
	return s.captchaAccepted && s.finalCode != ""
}

func (s *Server) emitLoginRequestLog(state *loginRequestLogState, c *fiber.Ctx) {
	if state == nil || !state.shouldLog() {
		return
	}
	now := time.Now()
	cost := requestCostSnapshot(c, now)
	stagePairs := make([]string, 0, len(state.stages))
	keys := make([]string, 0, len(state.stages))
	for key := range state.stages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		stagePairs = append(stagePairs, fmt.Sprintf("%s=%.3f", key, state.stages[key]))
	}
	eventName := "login request slow"
	if state.finalStatus >= 500 || (state.captchaAccepted && state.finalCode != "" && state.finalCode != codeLoginIPVerifyRequired) {
		eventName = "login request failed"
	}
	log.Printf(
		"%s path=%s status=%d code=%s %s ip_hash=%s username_hash=%s user_id=%d captcha_accepted=%t ip_verify_required=%t session_rotated=%t stages=\"%s\"",
		eventName,
		state.path,
		state.finalStatus,
		strings.TrimSpace(state.finalCode),
		requestCostLogFields(cost),
		state.clientIPHash,
		state.usernameHash,
		state.userID,
		state.captchaAccepted,
		state.ipVerifyRequired,
		state.sessionRotated,
		strings.Join(stagePairs, " "),
	)
}

func shortHash(raw string) string {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:6])
}

func requestCostLogFields(cost reqmetrics.Cost) string {
	return fmt.Sprintf(
		"total_ms=%.3f pg_ms=%.3f pg_acquire_ms=%.3f redis_ms=%.3f nats_ms=%.3f auth_ms=%.3f auth_total_ms=%.3f auth_parse_ms=%.3f auth_db_ms=%.3f business_ms=%.3f server_ms=%.3f pool_acquired=%d pool_idle=%d pool_max=%d",
		cost.Total,
		cost.PG,
		cost.PGAcquire,
		cost.Redis,
		cost.NATS,
		cost.Auth,
		cost.Auth,
		cost.AuthParse,
		cost.AuthDB,
		cost.Business,
		cost.Server,
		cost.PoolAcquired,
		cost.PoolIdle,
		cost.PoolMax,
	)
}

func durationMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func (s *Server) recordWSPressure(now time.Time, mutate func(*wsPressureCounters)) {
	if s == nil {
		return
	}
	s.wsPressureMu.Lock()
	defer s.wsPressureMu.Unlock()
	if mutate != nil {
		mutate(&s.wsPressure)
	}
	if now.IsZero() {
		now = time.Now()
	}
	if now.Sub(s.wsPressureLastSummary) < wsPressureSummaryInterval {
		return
	}
	if s.wsPressure == (wsPressureCounters{}) {
		s.wsPressureLastSummary = now
		return
	}
	log.Printf(
		"ws broadcast pressure node=%s hub_clients=%d targeted_attempts=%d targeted_delivered=%d targeted_skipped=%d broadcast_attempts=%d broadcast_delivered=%d broadcast_skipped=%d battle_skipped_subscribed=%d queue_full_closes=%d slow_consumer_events=%d",
		s.wsNodeAdvertise,
		len(s.deps.Hub.Clients()),
		s.wsPressure.TargetedAttempts,
		s.wsPressure.TargetedDelivered,
		s.wsPressure.TargetedSkipped,
		s.wsPressure.BroadcastAttempts,
		s.wsPressure.BroadcastDelivered,
		s.wsPressure.BroadcastSkipped,
		s.wsPressure.BattleSkippedSubscribed,
		s.wsPressure.QueueFullCloses,
		s.wsPressure.SlowConsumerEvents,
	)
	s.wsPressure = wsPressureCounters{}
	s.wsPressureLastSummary = now
}

func (s *Server) recordWSSlowConsumer(now time.Time, subject string, err error) {
	if s == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.wsPressureMu.Lock()
	s.wsPressure.SlowConsumerEvents++
	s.wsSlowConsumerAt = append(s.wsSlowConsumerAt, now)
	s.pruneWSSlowConsumerLocked(now)
	s.wsPressureMu.Unlock()
	log.Printf("ws broadcast pressure node=%s event=slow_consumer subject=%s err=%v", s.wsNodeAdvertise, strings.TrimSpace(subject), err)
	s.recordWSPressure(now, nil)
}

func (s *Server) pruneWSSlowConsumerLocked(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	kept := s.wsSlowConsumerAt[:0]
	for _, item := range s.wsSlowConsumerAt {
		if now.Sub(item) <= readinessPressureWindow {
			kept = append(kept, item)
		}
	}
	s.wsSlowConsumerAt = kept
}

func (s *Server) readinessDegradedReasons(now time.Time) []string {
	if now.IsZero() {
		now = time.Now()
	}
	reasons := make([]string, 0, 4)
	if readinessStoreHealth == nil || s == nil || s.deps.Store == nil {
		reasons = append(reasons, "store_unavailable")
	} else {
		ctx, cancel := readinessContext(s)
		if err := readinessStoreHealth(s, ctx); err != nil {
			reasons = append(reasons, "store_unhealthy")
		}
		cancel()
	}
	if readinessAuthHealth == nil || s == nil || s.deps.AuthGate == nil {
		reasons = append(reasons, "auth_store_unavailable")
	} else {
		ctx, cancel := readinessContext(s)
		if err := readinessAuthHealth(s, ctx); err != nil {
			reasons = append(reasons, "auth_store_unhealthy")
		} else {
			acquired, _, maxConns := s.deps.AuthGate.PoolStats()
			if maxConns > 0 && acquired >= maxConns {
				reasons = append(reasons, "auth_pool_saturated")
			}
		}
		cancel()
	}
	if readinessRedisPing == nil || s == nil || s.deps.Redis == nil {
		reasons = append(reasons, "redis_unavailable")
	} else {
		ctx, cancel := readinessContext(s)
		if err := readinessRedisPing(s, ctx); err != nil {
			reasons = append(reasons, "redis_unhealthy")
		}
		cancel()
	}
	if s == nil || s.wsNATS == nil || !s.wsNATS.IsConnected() {
		reasons = append(reasons, "nats_unavailable")
	}
	if s != nil {
		s.wsPressureMu.Lock()
		s.pruneWSSlowConsumerLocked(now)
		slowConsumers := len(s.wsSlowConsumerAt)
		s.wsPressureMu.Unlock()
		if slowConsumers >= readinessWSSlowConsumerLimit {
			reasons = append(reasons, "ws_pressure")
		}
	}
	return reasons
}

func logAPIRequestSlow(c *fiber.Ctx, cost reqmetrics.Cost) {
	if c == nil {
		return
	}
	path := c.Path()
	if !strings.HasPrefix(path, "/api/") || cost.Total < durationMS(slowAPIRequestThreshold) {
		return
	}
	log.Printf(
		"api request slow method=%s path=%s status=%d %s",
		c.Method(),
		path,
		c.Response().StatusCode(),
		requestCostLogFields(cost),
	)
}
