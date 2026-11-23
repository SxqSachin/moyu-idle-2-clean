package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"mysecondwebgame/server/internal/store"

	"github.com/redis/go-redis/v9"
)

const arenaRecommendationRefreshStateBaseTTL = 24 * time.Hour

var (
	errArenaRecommendationRefreshCooldown    = errors.New("arena recommendations are on refresh cooldown")
	errArenaRecommendationRefreshUnavailable = errors.New("arena recommendation refresh cache is unavailable")

	arenaRecommendationRefreshScript = redis.NewScript(`
local now_ms = tonumber(ARGV[1])
local cooldown_ms = tonumber(ARGV[2])
local refreshed_at_ms = tonumber(redis.call("GET", KEYS[1]))

if refreshed_at_ms and now_ms < refreshed_at_ms + cooldown_ms then
	return -refreshed_at_ms
end

redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[3])
return now_ms
`)
)

type arenaRecommendationRefreshState struct {
	ServerTime  time.Time
	RefreshedAt *time.Time
	AvailableAt *time.Time
}

func arenaRecommendationRefreshKey(userID int64) string {
	return "arena:recommendation_refresh:v1:" + strconv.FormatInt(userID, 10)
}

func buildArenaRecommendationRefreshState(serverTime time.Time, refreshedAt *time.Time, cooldown time.Duration) arenaRecommendationRefreshState {
	state := arenaRecommendationRefreshState{ServerTime: serverTime.UTC()}
	if refreshedAt == nil {
		return state
	}
	refreshed := refreshedAt.UTC()
	available := refreshed.Add(cooldown)
	state.RefreshedAt = &refreshed
	state.AvailableAt = &available
	return state
}

func validateArenaRecommendationRefreshInput(userID int64, cooldown time.Duration) error {
	if userID <= 0 || cooldown <= 0 || cooldown.Milliseconds() <= 0 {
		return store.ErrBattleRoomConfigInvalid
	}
	return nil
}

func (s *Server) getArenaRecommendationRefreshState(ctx context.Context, userID int64, cooldown time.Duration) (arenaRecommendationRefreshState, error) {
	if err := validateArenaRecommendationRefreshInput(userID, cooldown); err != nil {
		return arenaRecommendationRefreshState{}, err
	}
	if s.deps.Redis == nil {
		return arenaRecommendationRefreshState{}, errArenaRecommendationRefreshUnavailable
	}

	raw, err := s.deps.Redis.Get(ctx, arenaRecommendationRefreshKey(userID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return arenaRecommendationRefreshState{}, fmt.Errorf("read arena recommendation refresh state: %w", err)
	}
	serverTime, timeErr := s.deps.Redis.Time(ctx).Result()
	if timeErr != nil {
		return arenaRecommendationRefreshState{}, fmt.Errorf("read redis server time: %w", timeErr)
	}
	if errors.Is(err, redis.Nil) {
		return buildArenaRecommendationRefreshState(serverTime, nil, cooldown), nil
	}
	refreshedAtMS, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil || refreshedAtMS <= 0 {
		return arenaRecommendationRefreshState{}, fmt.Errorf("invalid arena recommendation refresh timestamp %q", raw)
	}
	refreshedAt := time.UnixMilli(refreshedAtMS)
	return buildArenaRecommendationRefreshState(serverTime, &refreshedAt, cooldown), nil
}

func (s *Server) refreshArenaRecommendations(ctx context.Context, userID int64, cooldown time.Duration) (arenaRecommendationRefreshState, error) {
	if err := validateArenaRecommendationRefreshInput(userID, cooldown); err != nil {
		return arenaRecommendationRefreshState{}, err
	}
	if s.deps.Redis == nil {
		return arenaRecommendationRefreshState{}, errArenaRecommendationRefreshUnavailable
	}

	serverTime, err := s.deps.Redis.Time(ctx).Result()
	if err != nil {
		return arenaRecommendationRefreshState{}, fmt.Errorf("read redis server time: %w", err)
	}
	stateTTL := arenaRecommendationRefreshStateBaseTTL + cooldown
	refreshedAtMS, err := arenaRecommendationRefreshScript.Run(
		ctx,
		s.deps.Redis,
		[]string{arenaRecommendationRefreshKey(userID)},
		serverTime.UnixMilli(),
		cooldown.Milliseconds(),
		stateTTL.Milliseconds(),
	).Int64()
	if err != nil {
		return arenaRecommendationRefreshState{}, fmt.Errorf("update arena recommendation refresh state: %w", err)
	}
	if refreshedAtMS == 0 {
		return arenaRecommendationRefreshState{}, errors.New("redis returned an empty arena recommendation refresh timestamp")
	}
	if refreshedAtMS < 0 {
		refreshedAt := time.UnixMilli(-refreshedAtMS)
		return buildArenaRecommendationRefreshState(serverTime, &refreshedAt, cooldown), errArenaRecommendationRefreshCooldown
	}
	refreshedAt := time.UnixMilli(refreshedAtMS)
	return buildArenaRecommendationRefreshState(serverTime, &refreshedAt, cooldown), nil
}
