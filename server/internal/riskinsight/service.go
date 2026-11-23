package riskinsight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"mysecondwebgame/server/internal/store"
)

const (
	publicCacheTTL       = time.Hour
	publicStaleTTL       = 6 * time.Hour
	publicBuildLockTTL   = 30 * time.Second
	publicBuildWaitStep  = 120 * time.Millisecond
	publicBuildWaitTries = 10
	publicMaxItems       = 50
)

var ErrInvalidCategory = errors.New("invalid public risk category")

type Service struct {
	store *store.Store
	redis *redis.Client

	mu       sync.Mutex
	fallback map[string]fallbackCacheEntry
}

type fallbackCacheEntry struct {
	payload PublicNoticeboardSnapshot
	expires time.Time
}

func NewService(st *store.Store, rd *redis.Client) *Service {
	if st == nil {
		return nil
	}
	return &Service{store: st, redis: rd, fallback: make(map[string]fallbackCacheEntry)}
}

func NormalizeCategory(category string) (string, error) {
	category = strings.TrimSpace(strings.ToLower(category))
	switch category {
	case "", CategoryAbnormalTrade:
		return CategoryAbnormalTrade, nil
	case CategoryLinkedAccounts, CategoryBanned, CategoryAll:
		return category, nil
	default:
		return "", ErrInvalidCategory
	}
}

func (s *Service) GetPublicNoticeboard(ctx context.Context, category string) (PublicNoticeboardSnapshot, error) {
	category, err := NormalizeCategory(category)
	if err != nil {
		return PublicNoticeboardSnapshot{}, err
	}
	if s == nil || s.store == nil {
		return emptySnapshot(category, time.Now().UTC(), false), nil
	}
	if s.redis == nil {
		return s.getPublicNoticeboardFallback(ctx, category)
	}
	if cached, ok := s.readRedisSnapshot(ctx, publicCacheKey(category)); ok {
		return cached, nil
	}
	locked, token := s.tryBuildLock(ctx, category)
	if locked {
		defer s.releaseBuildLock(ctx, category, token)
		return s.buildAndCachePublicNoticeboard(ctx, category)
	}
	for i := 0; i < publicBuildWaitTries; i++ {
		select {
		case <-ctx.Done():
			return PublicNoticeboardSnapshot{}, ctx.Err()
		case <-time.After(publicBuildWaitStep):
		}
		if cached, ok := s.readRedisSnapshot(ctx, publicCacheKey(category)); ok {
			return cached, nil
		}
	}
	if stale, ok := s.readRedisSnapshot(ctx, publicStaleKey(category)); ok {
		return markSnapshotStale(stale), nil
	}
	return emptySnapshot(category, time.Now().UTC(), false), nil
}

func (s *Service) getPublicNoticeboardFallback(ctx context.Context, category string) (PublicNoticeboardSnapshot, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	entry, ok := s.fallback[category]
	s.mu.Unlock()
	if ok && now.Before(entry.expires) {
		return entry.payload, nil
	}
	snapshot, err := s.buildPublicNoticeboard(ctx, category, now)
	if err != nil {
		if ok {
			return markSnapshotStale(entry.payload), nil
		}
		return emptySnapshot(category, now, false), nil
	}
	s.mu.Lock()
	s.fallback[category] = fallbackCacheEntry{payload: snapshot, expires: now.Add(publicCacheTTL)}
	s.mu.Unlock()
	return snapshot, nil
}

func (s *Service) buildAndCachePublicNoticeboard(ctx context.Context, category string) (PublicNoticeboardSnapshot, error) {
	now := time.Now().UTC()
	snapshot, err := s.buildPublicNoticeboard(ctx, category, now)
	if err != nil {
		if stale, ok := s.readRedisSnapshot(ctx, publicStaleKey(category)); ok {
			return markSnapshotStale(stale), nil
		}
		return emptySnapshot(category, now, false), nil
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return snapshot, nil
	}
	_ = s.redis.Set(ctx, publicCacheKey(category), raw, publicCacheTTL).Err()
	_ = s.redis.Set(ctx, publicStaleKey(category), raw, publicStaleTTL).Err()
	return snapshot, nil
}

func (s *Service) buildPublicNoticeboard(ctx context.Context, category string, now time.Time) (PublicNoticeboardSnapshot, error) {
	if category == CategoryAll {
		return s.buildAllPublicNoticeboard(ctx, now)
	}
	windowDays := 30
	if category == CategoryBanned {
		windowDays = 90
	}
	windowStartedAt := now.AddDate(0, 0, -windowDays)
	var items []PublicNoticeboardEntry
	var err error
	switch category {
	case CategoryAbnormalTrade:
		items, err = s.buildAbnormalTradeItems(ctx, windowStartedAt, now)
	case CategoryLinkedAccounts:
		items, err = s.buildLinkedAccountItems(ctx, windowStartedAt, now)
	case CategoryBanned:
		items, err = s.buildBannedItems(ctx, windowStartedAt, now)
	default:
		return PublicNoticeboardSnapshot{}, ErrInvalidCategory
	}
	if err != nil {
		return PublicNoticeboardSnapshot{}, err
	}
	expiresAt := now.Add(publicCacheTTL)
	for idx := range items {
		items[idx].WindowStartedAt = windowStartedAt
		items[idx].WindowEndedAt = now
		items[idx].GeneratedAt = now
		items[idx].ExpiresAt = expiresAt
	}
	return PublicNoticeboardSnapshot{
		Category:        category,
		Items:           items,
		WindowStartedAt: windowStartedAt,
		WindowEndedAt:   now,
		GeneratedAt:     now,
		ExpiresAt:       expiresAt,
		Stale:           false,
	}, nil
}

func (s *Service) buildAllPublicNoticeboard(ctx context.Context, now time.Time) (PublicNoticeboardSnapshot, error) {
	tradeWindowStartedAt := now.AddDate(0, 0, -30)
	bannedWindowStartedAt := now.AddDate(0, 0, -90)
	tradeItems, err := s.buildAbnormalTradeItems(ctx, tradeWindowStartedAt, now)
	if err != nil {
		return PublicNoticeboardSnapshot{}, err
	}
	linkedItems, err := s.buildLinkedAccountItems(ctx, tradeWindowStartedAt, now)
	if err != nil {
		return PublicNoticeboardSnapshot{}, err
	}
	bannedItems, err := s.buildBannedItems(ctx, bannedWindowStartedAt, now)
	if err != nil {
		return PublicNoticeboardSnapshot{}, err
	}
	expiresAt := now.Add(publicCacheTTL)
	items := make([]PublicNoticeboardEntry, 0, len(tradeItems)+len(linkedItems)+len(bannedItems))
	items = append(items, tradeItems...)
	items = append(items, linkedItems...)
	items = append(items, bannedItems...)
	for idx := range items {
		if items[idx].Category == CategoryBanned {
			items[idx].WindowStartedAt = bannedWindowStartedAt
		} else {
			items[idx].WindowStartedAt = tradeWindowStartedAt
		}
		items[idx].WindowEndedAt = now
		items[idx].GeneratedAt = now
		items[idx].ExpiresAt = expiresAt
	}
	return PublicNoticeboardSnapshot{
		Category:        CategoryAll,
		Items:           items,
		WindowStartedAt: bannedWindowStartedAt,
		WindowEndedAt:   now,
		GeneratedAt:     now,
		ExpiresAt:       expiresAt,
		Stale:           false,
	}, nil
}

func (s *Service) buildAbnormalTradeItems(ctx context.Context, since, now time.Time) ([]PublicNoticeboardEntry, error) {
	rows, err := s.store.Pool().Query(ctx, `
		WITH pair_evidence AS (
			SELECT
				LEAST(e.user_id, e.peer_user_id) AS user_a,
				GREATEST(e.user_id, e.peer_user_id) AS user_b,
				COUNT(*)::INT AS evidence_count,
				SUM(e.score)::INT AS total_score,
				MAX(e.last_seen_at) AS last_seen_at
			FROM risk_account_evidence e
			WHERE e.last_seen_at >= $1
			  AND e.evidence_type = ANY($2)
			  AND e.score >= 20
			GROUP BY LEAST(e.user_id, e.peer_user_id), GREATEST(e.user_id, e.peer_user_id)
		),
		pair_transfers AS (
			SELECT
				LEAST(t.from_user_id, t.to_user_id) AS user_a,
				GREATEST(t.from_user_id, t.to_user_id) AS user_b,
				COUNT(*)::INT AS transfer_count,
				COALESCE(SUM(t.net_amount), 0) AS transfer_amount,
				COALESCE(SUM(CASE WHEN t.price_anchor > 0 AND (
					(t.unit_price::numeric / GREATEST(t.price_anchor, 1)::numeric) < 0.4
					OR (t.unit_price::numeric / GREATEST(t.price_anchor, 1)::numeric) > 2.5
				) THEN 1 ELSE 0 END), 0)::INT AS abnormal_price_count,
				MAX(t.occurred_at) AS last_transfer_at
			FROM risk_account_transfers t
			WHERE t.occurred_at >= $1
			GROUP BY LEAST(t.from_user_id, t.to_user_id), GREATEST(t.from_user_id, t.to_user_id)
		)
		SELECT pe.user_a, COALESCE(ua.username, ''), pe.user_b, COALESCE(ub.username, ''),
		       pe.evidence_count, pe.total_score,
		       COALESCE(pt.transfer_count, 0), COALESCE(pt.abnormal_price_count, 0),
		       GREATEST(pe.last_seen_at, COALESCE(pt.last_transfer_at, pe.last_seen_at)) AS last_seen_at
		FROM pair_evidence pe
		JOIN users ua ON ua.id = pe.user_a
		JOIN users ub ON ub.id = pe.user_b
		LEFT JOIN pair_transfers pt ON pt.user_a = pe.user_a AND pt.user_b = pe.user_b
		WHERE pe.total_score >= 35 OR COALESCE(pt.abnormal_price_count, 0) >= 2 OR COALESCE(pt.transfer_count, 0) >= 5
		ORDER BY pe.total_score DESC, COALESCE(pt.abnormal_price_count, 0) DESC, last_seen_at DESC
		LIMIT $3
	`, since, []string{"market_trade", "auction_trade", "repeated_trade", "trade_loop", "one_way_transfer", "abnormal_price"}, publicMaxItems)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PublicNoticeboardEntry, 0, publicMaxItems)
	for rows.Next() {
		var userA, userB int64
		var usernameA, usernameB string
		var evidenceCount, totalScore, transferCount, abnormalPriceCount int
		var lastSeenAt time.Time
		if err := rows.Scan(&userA, &usernameA, &userB, &usernameB, &evidenceCount, &totalScore, &transferCount, &abnormalPriceCount, &lastSeenAt); err != nil {
			return nil, err
		}
		items = append(items, PublicNoticeboardEntry{
			Category:       CategoryAbnormalTrade,
			Users:          []PublicUser{{UserID: userA, Username: usernameA}, {UserID: userB, Username: usernameB}},
			Status:         "under_review",
			ConfidenceBand: confidenceBand(totalScore, evidenceCount, abnormalPriceCount),
			SummaryText:    publicSummary(CategoryAbnormalTrade, evidenceCount, transferCount),
		})
	}
	return items, rows.Err()
}

func (s *Service) buildLinkedAccountItems(ctx context.Context, since, now time.Time) ([]PublicNoticeboardEntry, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT rc.cluster_id,
		       rc.risk_score,
		       rc.status,
		       rc.updated_at,
		       jsonb_agg(
		         jsonb_build_object('userId', u.id, 'username', COALESCE(u.username, ''))
		         ORDER BY rcm.member_score DESC, u.id ASC
		       ) AS users_jsonb
		FROM risk_clusters rc
		JOIN risk_cluster_members rcm ON rcm.cluster_id = rc.cluster_id
		JOIN users u ON u.id = rcm.user_id
		WHERE rc.updated_at >= $1
		  AND rc.member_count >= 2
		  AND rc.risk_score >= 50
		  AND rc.status <> 'released'
		GROUP BY rc.cluster_id, rc.risk_score, rc.status, rc.updated_at
		ORDER BY rc.risk_score DESC, rc.updated_at DESC, rc.cluster_id DESC
		LIMIT $2
	`, since, publicMaxItems)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PublicNoticeboardEntry, 0, publicMaxItems)
	for rows.Next() {
		var clusterID int64
		var riskScore int
		var status string
		var updatedAt time.Time
		var rawUsers []byte
		if err := rows.Scan(&clusterID, &riskScore, &status, &updatedAt, &rawUsers); err != nil {
			return nil, err
		}
		var users []PublicUser
		if err := json.Unmarshal(rawUsers, &users); err != nil {
			return nil, err
		}
		if len(users) < 2 {
			continue
		}
		items = append(items, PublicNoticeboardEntry{
			Category:       CategoryLinkedAccounts,
			Users:          users,
			Status:         publicClusterStatus(status),
			ConfidenceBand: confidenceBand(riskScore, len(users), 0),
			SummaryText:    publicSummary(CategoryLinkedAccounts, len(users), 0),
		})
	}
	return items, rows.Err()
}

func (s *Service) buildBannedItems(ctx context.Context, since, now time.Time) ([]PublicNoticeboardEntry, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT u.id, COALESCE(u.username, ''), u.ban_until, u.ban_is_permanent, COALESCE(rc.updated_at, u.created_at) AS handled_at
		FROM users u
		LEFT JOIN LATERAL (
			SELECT updated_at
			FROM risk_review_cases
			WHERE user_id = u.id OR cluster_id IN (
				SELECT cluster_id FROM risk_cluster_members WHERE user_id = u.id
			)
			ORDER BY updated_at DESC
			LIMIT 1
		) rc ON TRUE
		WHERE btrim(u.ban_reason) <> ''
		  AND (u.ban_is_permanent OR u.ban_until IS NULL OR u.ban_until > now())
		  AND COALESCE(rc.updated_at, u.created_at) >= $1
		ORDER BY handled_at DESC, u.id DESC
		LIMIT $2
	`, since, publicMaxItems)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]PublicNoticeboardEntry, 0, publicMaxItems)
	for rows.Next() {
		var userID int64
		var username string
		var banUntil *time.Time
		var permanent bool
		var handledAt time.Time
		if err := rows.Scan(&userID, &username, &banUntil, &permanent, &handledAt); err != nil {
			return nil, err
		}
		status := "restricted"
		if permanent || banUntil == nil {
			status = "handled"
		}
		items = append(items, PublicNoticeboardEntry{
			Category:       CategoryBanned,
			Users:          []PublicUser{{UserID: userID, Username: username}},
			Status:         status,
			ConfidenceBand: "confirmed",
			SummaryText:    publicSummary(CategoryBanned, 1, 0),
		})
	}
	return items, rows.Err()
}

func publicSummary(category string, evidenceCount, transferCount int) string {
	switch category {
	case CategoryAbnormalTrade:
		if transferCount >= 5 {
			return "近期交易行为存在高置信异常，已进入公示与复核流程。"
		}
		return "近期交易行为存在高置信异常。"
	case CategoryLinkedAccounts:
		if evidenceCount >= 3 {
			return "该账号组近期协作特征异常，已进入公示与复核流程。"
		}
		return "近期账号协作特征异常，已进入公示与复核流程。"
	case CategoryBanned:
		return "该玩家已被处理。"
	default:
		return "近期行为存在高置信异常。"
	}
}

func publicClusterStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "manual_review", "restricted", "sealed":
		return "restricted"
	default:
		return "under_review"
	}
}

func confidenceBand(score, evidenceCount, abnormalCount int) string {
	if score >= 80 || evidenceCount >= 3 || abnormalCount >= 2 {
		return "confirmed"
	}
	return "high"
}

func emptySnapshot(category string, now time.Time, stale bool) PublicNoticeboardSnapshot {
	category, _ = NormalizeCategory(category)
	return PublicNoticeboardSnapshot{
		Category:        category,
		Items:           []PublicNoticeboardEntry{},
		WindowStartedAt: now,
		WindowEndedAt:   now,
		GeneratedAt:     now,
		ExpiresAt:       now.Add(publicCacheTTL),
		Stale:           stale,
	}
}

func markSnapshotStale(snapshot PublicNoticeboardSnapshot) PublicNoticeboardSnapshot {
	snapshot.Stale = true
	for idx := range snapshot.Items {
		snapshot.Items[idx].Stale = true
	}
	return snapshot
}

func (s *Service) readRedisSnapshot(ctx context.Context, key string) (PublicNoticeboardSnapshot, bool) {
	raw, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		return PublicNoticeboardSnapshot{}, false
	}
	var snapshot PublicNoticeboardSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return PublicNoticeboardSnapshot{}, false
	}
	if snapshot.Items == nil {
		snapshot.Items = []PublicNoticeboardEntry{}
	}
	sort.SliceStable(snapshot.Items, func(i, j int) bool {
		return snapshot.Items[i].GeneratedAt.After(snapshot.Items[j].GeneratedAt)
	})
	return snapshot, true
}

func (s *Service) tryBuildLock(ctx context.Context, category string) (bool, string) {
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := s.redis.SetNX(ctx, publicBuildLockKey(category), token, publicBuildLockTTL).Result()
	if err != nil {
		return false, ""
	}
	return ok, token
}

func (s *Service) releaseBuildLock(ctx context.Context, category, token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`
	_ = s.redis.Eval(ctx, script, []string{publicBuildLockKey(category)}, token).Err()
}

func publicCacheKey(category string) string {
	return "risk:public_noticeboard:v1:" + category
}

func publicStaleKey(category string) string {
	return "risk:public_noticeboard:v1:stale:" + category
}

func publicBuildLockKey(category string) string {
	return "risk:public_noticeboard:build_lock:" + category
}
