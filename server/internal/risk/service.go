package risk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mysecondwebgame/server/internal/store"
)

const (
	CapabilityTrade         = "trade"
	CapabilityTradeTransfer = "trade_transfer"
	CapabilityTeamJoin      = "team_join"
	CapabilityTeamInvite    = "team_invite"

	levelObserve    = "observe"
	levelGuarded    = "guarded"
	levelRestricted = "restricted"
	levelSealed     = "sealed"
	levelBanned     = "banned"
)

var ErrCapabilityRestricted = errors.New("risk capability restricted")
var ErrClusterNotFound = errors.New("risk cluster not found")
var ErrInvalidDisposition = errors.New("invalid risk disposition")

type Service struct {
	store          *store.Store
	passwordPepper string
}

func NewService(st *store.Store, passwordPepper string) *Service {
	if st == nil {
		return nil
	}
	return &Service{store: st, passwordPepper: strings.TrimSpace(passwordPepper)}
}

func (s *Service) RecordSignal(ctx context.Context, input SignalInput) error {
	if s == nil || s.store == nil || input.UserID <= 0 {
		return nil
	}
	normalized := normalizeSignalInput(input)
	return s.store.WithTx(ctx, func(tx pgx.Tx) error {
		extraJSON, err := json.Marshal(normalized.Extra)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO risk_account_signals(
				user_id, event_type, occurred_at, ip, ip_prefix, asn, country_code,
				fingerprint_id, fingerprint_version, device_family_hash, browser_family_hash, runtime_family_hash,
				cookie_id, local_device_id, browser_install_id, session_chain_id,
				ua_hash, accept_language, timezone, screen_hash, canvas_hash, webgl_hash, audio_hash,
				cpu_hint, memory_hint, platform_hint, captcha_score, extra_jsonb
			) VALUES (
				$1, $2, now(), $3, $4, $5, $6,
				$7, $8, $9, $10, $11,
				$12, $13, $14, $15,
				$16, $17, $18, $19, $20, $21, $22,
				$23, $24, $25, $26, $27::jsonb
			)
		`,
			normalized.UserID, normalized.EventType, normalized.IP, normalized.ExtractIPPrefix(), normalized.ASN, normalized.CountryCode,
			normalized.DeviceProfile.FingerprintID, normalized.DeviceProfile.FingerprintVersion, normalized.DeviceProfile.DeviceFamilyHash,
			normalized.DeviceProfile.BrowserFamilyHash, normalized.DeviceProfile.RuntimeFamilyHash,
			normalized.DeviceProfile.CookieID, normalized.DeviceProfile.LocalDeviceID, normalized.DeviceProfile.BrowserInstallID, normalized.DeviceProfile.SessionChainID,
			hashText(normalized.UserAgent), normalized.AcceptLanguage, normalized.DeviceProfile.Timezone, normalized.DeviceProfile.ScreenBucket,
			normalized.DeviceProfile.CanvasHash, normalized.DeviceProfile.WebGLHash, normalized.DeviceProfile.AudioHash,
			normalized.DeviceProfile.CPUHint, normalized.DeviceProfile.MemoryHint, normalized.DeviceProfile.PlatformHint,
			normalized.DeviceProfile.CaptchaScore, string(extraJSON),
		); err != nil {
			return err
		}
		if digest := s.digestPasswordProof(normalized.PasswordRaw); digest != "" {
			if _, err := tx.Exec(ctx, `
				INSERT INTO risk_password_evidence(password_hash_digest, user_id, first_seen_at, last_seen_at)
				VALUES($1, $2, now(), now())
				ON CONFLICT(password_hash_digest, user_id)
				DO UPDATE SET last_seen_at = now()
			`, digest, normalized.UserID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ResolveUserContext(ctx context.Context, userID int64) (UserContext, error) {
	if s == nil || s.store == nil || userID <= 0 {
		return UserContext{UserID: userID, RiskLevel: levelObserve, ClusterStatus: "active"}, nil
	}
	row := s.store.Pool().QueryRow(ctx, `
		SELECT COALESCE(rcm.cluster_id, 0), COALESCE(rc.risk_score, 0), COALESCE(rc.risk_level, 'observe'), COALESCE(rc.status, 'active')
		FROM users u
		LEFT JOIN risk_cluster_members rcm ON rcm.user_id = u.id
		LEFT JOIN risk_clusters rc ON rc.cluster_id = rcm.cluster_id
		WHERE u.id = $1
	`, userID)
	var clusterID int64
	var ctxOut UserContext
	ctxOut.UserID = userID
	if err := row.Scan(&clusterID, &ctxOut.RiskScore, &ctxOut.RiskLevel, &ctxOut.ClusterStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserContext{UserID: userID, RiskLevel: levelObserve, ClusterStatus: "active"}, nil
		}
		return UserContext{}, err
	}
	if clusterID > 0 {
		ctxOut.ClusterID = &clusterID
	}
	blocks, err := s.listBlocks(ctx, userID)
	if err != nil {
		return UserContext{}, err
	}
	ctxOut.RestrictedCapabilities = blocks
	return ctxOut, nil
}

func (s *Service) AssertCapabilityAllowed(ctx context.Context, userID int64, capability string) error {
	if s == nil || s.store == nil || userID <= 0 || strings.TrimSpace(capability) == "" {
		return nil
	}
	var count int
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT COUNT(1)
		FROM risk_capability_blocks
		WHERE user_id = $1
		  AND capability = $2
		  AND (expires_at IS NULL OR expires_at > now())
	`, userID, strings.TrimSpace(strings.ToLower(capability))).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrCapabilityRestricted
	}
	return nil
}

func (s *Service) RecomputeImpactedCluster(ctx context.Context, userID int64) error {
	if s == nil || s.store == nil || userID <= 0 {
		return nil
	}
	return s.store.WithTx(ctx, func(tx pgx.Tx) error {
		signals, err := s.loadSignalsForUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		if len(signals) == 0 {
			return nil
		}
		peers, err := s.loadCandidatePeerSignals(ctx, tx, userID, signals)
		if err != nil {
			return err
		}
		evidenceMap, err := s.buildEvidence(ctx, tx, userID, signals, peers)
		if err != nil {
			return err
		}
		if err := s.replaceUserEvidence(ctx, tx, userID, evidenceMap); err != nil {
			return err
		}
		component, err := s.collectClusterComponent(ctx, tx, userID)
		if err != nil {
			return err
		}
		if len(component) == 0 {
			component = []int64{userID}
		}
		return s.upsertClusterState(ctx, tx, component)
	})
}

func (s *Service) AdminGetUserRiskDetail(ctx context.Context, userID int64) (UserDetail, error) {
	var detail UserDetail
	if s == nil || s.store == nil || userID <= 0 {
		return detail, store.ErrUserNotFound
	}
	var userInfo UserDetailInfo
	userInfo.UserID = userID
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT COALESCE(TRIM(username), ''), COALESCE(TRIM(email), ''), created_at
		FROM users WHERE id = $1
	`, userID).Scan(&userInfo.Username, &userInfo.Email, &userInfo.CreatedAt); err != nil {
		return detail, err
	}
	ctxRow, err := s.ResolveUserContext(ctx, userID)
	if err != nil {
		return detail, err
	}
	detail.User = userInfo
	detail.Context = ctxRow
	detail.Blocks = ctxRow.RestrictedCapabilities
	if detail.Signals, err = s.listSignals(ctx, userID, 30); err != nil {
		return detail, err
	}
	if detail.Transfers, err = s.listTransfers(ctx, userID, 40); err != nil {
		return detail, err
	}
	if detail.TransferPeers, err = s.listTransferPeers(ctx, userID, 20); err != nil {
		return detail, err
	}
	if detail.Evidences, err = s.listEvidence(ctx, userID, 100); err != nil {
		return detail, err
	}
	if detail.LinkedUsers, err = s.listLinkedUsers(ctx, userID, 50); err != nil {
		return detail, err
	}
	if detail.ReviewCases, err = s.listReviewCasesForUser(ctx, userID); err != nil {
		return detail, err
	}
	return detail, nil
}

func (s *Service) AdminGetClusterDetail(ctx context.Context, clusterID int64) (ClusterDetail, error) {
	var out ClusterDetail
	if s == nil || s.store == nil || clusterID <= 0 {
		return out, ErrClusterNotFound
	}
	var primaryID *int64
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT cluster_id, risk_score, risk_level, status, member_count, primary_user_id
		FROM risk_clusters
		WHERE cluster_id = $1
	`, clusterID).Scan(&out.ClusterID, &out.RiskScore, &out.RiskLevel, &out.Status, &out.MemberCount, &primaryID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, ErrClusterNotFound
		}
		return out, err
	}
	if primaryID != nil && *primaryID > 0 {
		username, _ := s.store.GetUsernameByUserID(ctx, *primaryID)
		out.PrimaryUser = &UserSummary{UserID: *primaryID, Username: username}
	}
	rows, err := s.store.Pool().Query(ctx, `
		SELECT m.user_id, COALESCE(u.username, ''), m.member_score, m.reason_summary, m.joined_at,
		       COALESCE(TRIM(u.ban_reason), '') <> '' AS is_banned
		FROM risk_cluster_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.cluster_id = $1
		ORDER BY m.member_score DESC, m.user_id ASC
	`, clusterID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ClusterMember
		if err := rows.Scan(&item.User.UserID, &item.User.Username, &item.MemberScore, &item.ReasonSummary, &item.JoinedAt, &item.IsBanned); err != nil {
			return out, err
		}
		out.Members = append(out.Members, item)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	out.ReviewCases, err = s.listReviewCasesForCluster(ctx, clusterID)
	return out, err
}

func (s *Service) AdminSearch(ctx context.Context, q string, limit int) ([]SearchResult, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	q = strings.TrimSpace(q)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.store.Pool().Query(ctx, `
		WITH latest_signal AS (
			SELECT DISTINCT ON (user_id)
				user_id, local_device_id, fingerprint_id, ip, occurred_at
			FROM risk_account_signals
			ORDER BY user_id, occurred_at DESC
		)
		SELECT u.id, u.username, rcm.cluster_id, COALESCE(rc.risk_level, 'observe'), COALESCE(rc.risk_score, 0),
		       COALESCE(ls.local_device_id, ''), COALESCE(ls.fingerprint_id, ''), COALESCE(ls.ip, ''), ls.occurred_at,
		       COALESCE(TRIM(u.ban_reason), '') <> '' AS is_banned
		FROM users u
		LEFT JOIN latest_signal ls ON ls.user_id = u.id
		LEFT JOIN risk_cluster_members rcm ON rcm.user_id = u.id
		LEFT JOIN risk_clusters rc ON rc.cluster_id = rcm.cluster_id
		WHERE CAST(u.id AS TEXT) = $1
		   OR lower(u.username) LIKE '%' || lower($1) || '%'
		   OR COALESCE(ls.local_device_id, '') = $1
		   OR COALESCE(ls.fingerprint_id, '') = $1
		   OR COALESCE(ls.ip, '') = $1
		ORDER BY COALESCE(ls.occurred_at, now()) DESC, u.id DESC
		LIMIT $2
	`, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SearchResult, 0, limit)
	for rows.Next() {
		var item SearchResult
		if err := rows.Scan(&item.UserID, &item.Username, &item.ClusterID, &item.RiskLevel, &item.RiskScore, &item.LocalDeviceID, &item.FingerprintID, &item.IP, &item.OccurredAt, &item.IsBanned); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) AdminApplyDisposition(ctx context.Context, req DispositionRequest) error {
	if s == nil || s.store == nil {
		return nil
	}
	action := strings.TrimSpace(strings.ToLower(req.Action))
	if req.ClusterID <= 0 {
		return ErrClusterNotFound
	}
	switch action {
	case "recompute":
		memberIDs, err := s.listClusterMemberIDs(ctx, req.ClusterID)
		if err != nil {
			return err
		}
		for _, userID := range memberIDs {
			if err := s.RecomputeImpactedCluster(ctx, userID); err != nil {
				return err
			}
		}
		return nil
	case "restrict", "release", "ban":
	default:
		return ErrInvalidDisposition
	}
	memberIDs, err := s.listClusterMemberIDs(ctx, req.ClusterID)
	if err != nil {
		return err
	}
	if len(memberIDs) == 0 {
		return ErrClusterNotFound
	}
	switch action {
	case "ban":
		for _, userID := range memberIDs {
			if err := s.store.AdminBanUser(ctx, store.AdminUserBanInput{
				UserID:      userID,
				Reason:      "abnormal_trade",
				IsPermanent: true,
			}); err != nil {
				return err
			}
		}
		return s.updateReviewCase(ctx, req.ClusterID, req.OperatorName, req.OperatorNote, "resolved", "banned")
	case "release":
		return s.store.WithTx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM risk_capability_blocks WHERE cluster_id = $1`, req.ClusterID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE risk_clusters
				SET risk_level = 'observe', status = 'released', risk_score = 0, updated_at = now()
				WHERE cluster_id = $1
			`, req.ClusterID); err != nil {
				return err
			}
			return s.insertReviewCaseTx(ctx, tx, req.ClusterID, nil, "resolved", "released", req.OperatorName, req.OperatorNote)
		})
	case "restrict":
		return s.store.WithTx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `
				UPDATE risk_clusters
				SET risk_level = 'restricted', status = 'manual_review', risk_score = GREATEST(risk_score, 50), updated_at = now()
				WHERE cluster_id = $1
			`, req.ClusterID); err != nil {
				return err
			}
			if err := s.replaceCapabilityBlocksTx(ctx, tx, req.ClusterID, memberIDs, levelRestricted, "admin_manual", "admin_manual"); err != nil {
				return err
			}
			return s.insertReviewCaseTx(ctx, tx, req.ClusterID, nil, "open", "restricted", req.OperatorName, req.OperatorNote)
		})
	}
	return nil
}

func (s *Service) AdminListReviewCases(ctx context.Context, limit int) ([]ReviewCase, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.store.Pool().Query(ctx, `
		SELECT id, cluster_id, user_id, status, decision, operator_name, operator_note, created_at, updated_at
		FROM risk_review_cases
		ORDER BY updated_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ReviewCase, 0, limit)
	for rows.Next() {
		var item ReviewCase
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.UserID, &item.Status, &item.Decision, &item.OperatorName, &item.OperatorNote, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) AdminListHighRiskUsers(ctx context.Context, limit, offset int) (PaginatedResult[HighRiskUserItem], error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT COUNT(1)
		FROM risk_cluster_members rcm
		JOIN risk_clusters rc ON rc.cluster_id = rcm.cluster_id
		WHERE rc.risk_score >= 25
	`).Scan(&total); err != nil {
		return PaginatedResult[HighRiskUserItem]{}, err
	}
	rows, err := s.store.Pool().Query(ctx, `
		WITH latest_signal AS (
			SELECT DISTINCT ON (user_id) user_id, occurred_at
			FROM risk_account_signals
			ORDER BY user_id, occurred_at DESC
		)
		SELECT
			u.id,
			u.username,
			rcm.cluster_id,
			COALESCE(rc.risk_score, 0),
			COALESCE(rc.risk_level, 'observe'),
			COALESCE(rc.status, 'active'),
			ls.occurred_at,
			COALESCE(TRIM(u.ban_reason), '') <> '' AS is_banned
		FROM risk_cluster_members rcm
		JOIN users u ON u.id = rcm.user_id
		JOIN risk_clusters rc ON rc.cluster_id = rcm.cluster_id
		LEFT JOIN latest_signal ls ON ls.user_id = u.id
		WHERE rc.risk_score >= 25
		ORDER BY rc.risk_score DESC, ls.occurred_at DESC NULLS LAST, u.id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return PaginatedResult[HighRiskUserItem]{}, err
	}
	defer rows.Close()
	out := make([]HighRiskUserItem, 0, limit)
	for rows.Next() {
		var item HighRiskUserItem
		if err := rows.Scan(
			&item.User.UserID,
			&item.User.Username,
			&item.ClusterID,
			&item.RiskScore,
			&item.RiskLevel,
			&item.ClusterStatus,
			&item.LastSeenAt,
			&item.IsBanned,
		); err != nil {
			return PaginatedResult[HighRiskUserItem]{}, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return PaginatedResult[HighRiskUserItem]{}, err
	}
	return PaginatedResult[HighRiskUserItem]{Items: out, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) AdminListClusters(ctx context.Context, limit, offset int) (PaginatedResult[ClusterSummary], error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT COUNT(1)
		FROM risk_clusters
		WHERE member_count > 0
		  AND risk_score >= 25
	`).Scan(&total); err != nil {
		return PaginatedResult[ClusterSummary]{}, err
	}
	rows, err := s.store.Pool().Query(ctx, `
		SELECT rc.cluster_id, rc.risk_score, rc.risk_level, rc.status, rc.member_count, rc.primary_user_id, rc.updated_at
		FROM risk_clusters rc
		WHERE rc.member_count > 0
		  AND rc.risk_score >= 25
		ORDER BY rc.risk_score DESC, rc.updated_at DESC, rc.cluster_id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return PaginatedResult[ClusterSummary]{}, err
	}
	defer rows.Close()
	out := make([]ClusterSummary, 0, limit)
	for rows.Next() {
		var item ClusterSummary
		var primaryUserID *int64
		if err := rows.Scan(&item.ClusterID, &item.RiskScore, &item.RiskLevel, &item.Status, &item.MemberCount, &primaryUserID, &item.UpdatedAt); err != nil {
			return PaginatedResult[ClusterSummary]{}, err
		}
		if primaryUserID != nil && *primaryUserID > 0 {
			username, _ := s.store.GetUsernameByUserID(ctx, *primaryUserID)
			item.PrimaryUser = &UserSummary{UserID: *primaryUserID, Username: username}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return PaginatedResult[ClusterSummary]{}, err
	}
	return PaginatedResult[ClusterSummary]{Items: out, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) AdminResolveReviewCase(ctx context.Context, id int64, operatorName, operatorNote, decision string) error {
	if id <= 0 {
		return ErrInvalidDisposition
	}
	_, err := s.store.Pool().Exec(ctx, `
		UPDATE risk_review_cases
		SET status = 'resolved',
		    decision = $2,
		    operator_name = $3,
		    operator_note = $4,
		    updated_at = now()
		WHERE id = $1
	`, id, strings.TrimSpace(decision), strings.TrimSpace(operatorName), strings.TrimSpace(operatorNote))
	return err
}

type signalRecord struct {
	UserID            int64
	IP                string
	IPPrefix          string
	FingerprintID     string
	DeviceFamilyHash  string
	BrowserFamilyHash string
	RuntimeFamilyHash string
	CookieID          string
	LocalDeviceID     string
	SessionChainID    string
	Timezone          string
	Language          string
	OccurredAt        time.Time
}

type evidenceAccumulator struct {
	totalScore int
	maxSeenAt  time.Time
	minSeenAt  time.Time
	items      map[string]evidenceItem
}

type evidenceItem struct {
	strength string
	score    int
	payload  map[string]any
}

type transferSummary struct {
	peerUserID     int64
	totalCount     int
	marketCount    int
	auctionCount   int
	outCount       int
	inCount        int
	directOutCount int
	directInCount  int
	grossOut       int64
	grossIn        int64
	maxSingle      int64
	firstSeenAt    time.Time
	lastSeenAt     time.Time
	channels       map[string]int
	abnormalCount  int
}

func normalizeSignalInput(input SignalInput) SignalInput {
	input.EventType = strings.TrimSpace(strings.ToLower(input.EventType))
	if input.EventType == "" {
		input.EventType = "unknown"
	}
	input.IP = strings.TrimSpace(input.IP)
	input.UserAgent = strings.TrimSpace(input.UserAgent)
	input.AcceptLanguage = strings.TrimSpace(input.AcceptLanguage)
	input.ASN = strings.TrimSpace(input.ASN)
	input.CountryCode = strings.TrimSpace(strings.ToUpper(input.CountryCode))
	input.PasswordRaw = strings.TrimSpace(input.PasswordRaw)
	if input.Extra == nil {
		input.Extra = map[string]any{}
	}
	dp := input.DeviceProfile
	dp.FingerprintID = strings.TrimSpace(dp.FingerprintID)
	dp.FingerprintVersion = strings.TrimSpace(dp.FingerprintVersion)
	dp.ComponentsHash = strings.TrimSpace(dp.ComponentsHash)
	dp.DeviceFamilyHash = strings.TrimSpace(dp.DeviceFamilyHash)
	dp.BrowserFamilyHash = strings.TrimSpace(dp.BrowserFamilyHash)
	dp.RuntimeFamilyHash = strings.TrimSpace(dp.RuntimeFamilyHash)
	dp.CookieID = strings.TrimSpace(dp.CookieID)
	dp.LocalDeviceID = strings.TrimSpace(dp.LocalDeviceID)
	dp.BrowserInstallID = strings.TrimSpace(dp.BrowserInstallID)
	dp.SessionChainID = strings.TrimSpace(dp.SessionChainID)
	dp.Timezone = strings.TrimSpace(dp.Timezone)
	dp.Language = strings.TrimSpace(dp.Language)
	dp.ScreenBucket = strings.TrimSpace(dp.ScreenBucket)
	dp.CanvasHash = strings.TrimSpace(dp.CanvasHash)
	dp.WebGLHash = strings.TrimSpace(dp.WebGLHash)
	dp.AudioHash = strings.TrimSpace(dp.AudioHash)
	dp.PlatformHint = strings.TrimSpace(dp.PlatformHint)
	dp.CPUHint = strings.TrimSpace(dp.CPUHint)
	dp.MemoryHint = strings.TrimSpace(dp.MemoryHint)
	input.DeviceProfile = dp
	return input
}

func (in SignalInput) ExtractIPPrefix() string {
	return ipPrefix(in.IP)
}

func (s *Service) digestPasswordProof(passwordRaw string) string {
	passwordRaw = strings.TrimSpace(passwordRaw)
	if passwordRaw == "" {
		return ""
	}
	if s.passwordPepper == "" {
		sum := sha256.Sum256([]byte(passwordRaw))
		return hex.EncodeToString(sum[:])
	}
	mac := hmac.New(sha256.New, []byte(s.passwordPepper))
	_, _ = mac.Write([]byte(passwordRaw))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func ipPrefix(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", v4[0], v4[1], v4[2])
	}
	v6 := parsed.To16()
	if v6 == nil {
		return ""
	}
	return fmt.Sprintf("%x:%x:%x:%x::/64",
		uint16(v6[0])<<8|uint16(v6[1]),
		uint16(v6[2])<<8|uint16(v6[3]),
		uint16(v6[4])<<8|uint16(v6[5]),
		uint16(v6[6])<<8|uint16(v6[7]),
	)
}

func riskLevelForScore(score int) string {
	switch {
	case score >= 80:
		return levelSealed
	case score >= 50:
		return levelRestricted
	case score >= 25:
		return levelGuarded
	default:
		return levelObserve
	}
}

func (s *Service) loadSignalsForUser(ctx context.Context, tx pgx.Tx, userID int64) ([]signalRecord, error) {
	rows, err := tx.Query(ctx, `
		SELECT user_id, ip, ip_prefix, fingerprint_id, device_family_hash, browser_family_hash, runtime_family_hash,
		       cookie_id, local_device_id, session_chain_id, timezone, accept_language, occurred_at
		FROM risk_account_signals
		WHERE user_id = $1
		  AND occurred_at >= now() - interval '90 days'
		ORDER BY occurred_at DESC
		LIMIT 30
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]signalRecord, 0, 8)
	for rows.Next() {
		var row signalRecord
		if err := rows.Scan(&row.UserID, &row.IP, &row.IPPrefix, &row.FingerprintID, &row.DeviceFamilyHash, &row.BrowserFamilyHash, &row.RuntimeFamilyHash,
			&row.CookieID, &row.LocalDeviceID, &row.SessionChainID, &row.Timezone, &row.Language, &row.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func collectDistinct(values []signalRecord, pick func(signalRecord) string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		value := strings.TrimSpace(pick(item))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *Service) loadCandidatePeerSignals(ctx context.Context, tx pgx.Tx, userID int64, signals []signalRecord) (map[int64][]signalRecord, error) {
	localDeviceIDs := collectDistinct(signals, func(v signalRecord) string { return v.LocalDeviceID })
	fingerprints := collectDistinct(signals, func(v signalRecord) string { return v.FingerprintID })
	deviceFamilies := collectDistinct(signals, func(v signalRecord) string { return v.DeviceFamilyHash })
	cookies := collectDistinct(signals, func(v signalRecord) string { return v.CookieID })
	ipPrefixes := collectDistinct(signals, func(v signalRecord) string { return v.IPPrefix })
	sessionChains := collectDistinct(signals, func(v signalRecord) string { return v.SessionChainID })
	rows, err := tx.Query(ctx, `
		SELECT user_id, ip, ip_prefix, fingerprint_id, device_family_hash, browser_family_hash, runtime_family_hash,
		       cookie_id, local_device_id, session_chain_id, timezone, accept_language, occurred_at
		FROM risk_account_signals
		WHERE user_id <> $1
		  AND occurred_at >= now() - interval '90 days'
		  AND (
		    local_device_id = ANY($2)
		    OR fingerprint_id = ANY($3)
		    OR device_family_hash = ANY($4)
		    OR cookie_id = ANY($5)
		    OR ip_prefix = ANY($6)
		    OR session_chain_id = ANY($7)
		  )
	`, userID, localDeviceIDs, fingerprints, deviceFamilies, cookies, ipPrefixes, sessionChains)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]signalRecord)
	for rows.Next() {
		var row signalRecord
		if err := rows.Scan(&row.UserID, &row.IP, &row.IPPrefix, &row.FingerprintID, &row.DeviceFamilyHash, &row.BrowserFamilyHash, &row.RuntimeFamilyHash,
			&row.CookieID, &row.LocalDeviceID, &row.SessionChainID, &row.Timezone, &row.Language, &row.OccurredAt); err != nil {
			return nil, err
		}
		out[row.UserID] = append(out[row.UserID], row)
	}
	return out, rows.Err()
}

func (s *Service) buildEvidence(ctx context.Context, tx pgx.Tx, userID int64, signals []signalRecord, peers map[int64][]signalRecord) (map[int64]*evidenceAccumulator, error) {
	out := make(map[int64]*evidenceAccumulator)
	passwordPeers, err := s.loadPasswordPeers(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	for _, peerUserID := range passwordPeers {
		acc := ensureEvidenceAccumulator(out, peerUserID)
		acc.totalScore += 60
		acc.items["password_match"] = evidenceItem{strength: "strong", score: 60, payload: map[string]any{"match": true, "proof": "stable_password_pepper_hmac"}}
	}
	for peerUserID, peerSignals := range peers {
		acc := ensureEvidenceAccumulator(out, peerUserID)
		for _, self := range signals {
			for _, peer := range peerSignals {
				accumulateSignalEvidence(acc, self, peer)
			}
		}
		accumulateBehaviorEvidence(acc, signals, peerSignals)
	}
	transferSummaries, err := s.loadTransferSummaries(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	for peerUserID, summary := range transferSummaries {
		acc := ensureEvidenceAccumulator(out, peerUserID)
		if acc.minSeenAt.IsZero() || (!summary.firstSeenAt.IsZero() && summary.firstSeenAt.Before(acc.minSeenAt)) {
			acc.minSeenAt = summary.firstSeenAt
		}
		if summary.lastSeenAt.After(acc.maxSeenAt) {
			acc.maxSeenAt = summary.lastSeenAt
		}
		// Market: first 2 trades ignored; 3+ scores 6 pts each, cap 30
		if summary.marketCount >= 3 {
			marketScore := minInt((summary.marketCount-2)*6, 30)
			addEvidence(acc, "market_trade", "medium", marketScore, map[string]any{
				"count":      summary.marketCount,
				"grossOut":   summary.grossOut,
				"grossIn":    summary.grossIn,
				"maxSingle":  summary.maxSingle,
				"channelMix": summary.channels,
			})
		}
		// Auction: same threshold as market
		if summary.auctionCount >= 3 {
			auctionScore := minInt((summary.auctionCount-2)*6, 30)
			addEvidence(acc, "auction_trade", "medium", auctionScore, map[string]any{
				"count":      summary.auctionCount,
				"grossOut":   summary.grossOut,
				"grossIn":    summary.grossIn,
				"maxSingle":  summary.maxSingle,
				"channelMix": summary.channels,
			})
		}
		if summary.totalCount >= 3 {
			addEvidence(acc, "repeated_trade", "medium", minInt(10+(summary.totalCount-3)*3, 20), map[string]any{
				"count":     summary.totalCount,
				"grossOut":  summary.grossOut,
				"grossIn":   summary.grossIn,
				"outCount":  summary.outCount,
				"inCount":   summary.inCount,
				"maxSingle": summary.maxSingle,
			})
		}
		if summary.outCount > 0 && summary.inCount > 0 && summary.totalCount >= 3 {
			addEvidence(acc, "trade_loop", "medium", minInt(12+(summary.totalCount-3)*2, 20), map[string]any{
				"count":    summary.totalCount,
				"outCount": summary.outCount,
				"inCount":  summary.inCount,
			})
		}
		directTotal := summary.directOutCount + summary.directInCount
		if (summary.directOutCount == 0 || summary.directInCount == 0) && directTotal >= 2 {
			// only direct (non-market/auction) one-sided flows trigger this
			score := 25
			if directTotal >= 3 {
				score = 35
			}
			addEvidence(acc, "one_way_transfer", "strong", score, map[string]any{
				"count":          directTotal,
				"directOutCount": summary.directOutCount,
				"directInCount":  summary.directInCount,
				"grossOut":       summary.grossOut,
				"grossIn":        summary.grossIn,
				"maxSingle":      summary.maxSingle,
			})
		}
		// abnormal_price: extreme deviation only (< 40% or > 250% of base)
		if summary.abnormalCount > 0 {
			addEvidence(acc, "abnormal_price", "medium", minInt(20+(summary.abnormalCount-1)*5, 35), map[string]any{
				"count": summary.abnormalCount,
			})
		}
	}
	return out, nil
}

func ensureEvidenceAccumulator(out map[int64]*evidenceAccumulator, peerUserID int64) *evidenceAccumulator {
	acc, ok := out[peerUserID]
	if !ok {
		acc = &evidenceAccumulator{items: make(map[string]evidenceItem)}
		out[peerUserID] = acc
	}
	return acc
}

func accumulateSignalEvidence(acc *evidenceAccumulator, self signalRecord, peer signalRecord) {
	if acc.minSeenAt.IsZero() || peer.OccurredAt.Before(acc.minSeenAt) {
		acc.minSeenAt = peer.OccurredAt
	}
	if peer.OccurredAt.After(acc.maxSeenAt) {
		acc.maxSeenAt = peer.OccurredAt
	}
	if self.LocalDeviceID != "" && self.LocalDeviceID == peer.LocalDeviceID {
		addEvidence(acc, "local_device", "strong", 45, map[string]any{"localDeviceId": self.LocalDeviceID})
	}
	if self.CookieID != "" && self.CookieID == peer.CookieID {
		addEvidence(acc, "cookie", "medium", 20, map[string]any{"cookieId": self.CookieID})
	}
	if self.SessionChainID != "" && self.SessionChainID == peer.SessionChainID && self.DeviceFamilyHash != "" && self.DeviceFamilyHash == peer.DeviceFamilyHash {
		addEvidence(acc, "session_chain_device", "strong", 35, map[string]any{"sessionChainId": self.SessionChainID})
	}
	if self.FingerprintID != "" && self.FingerprintID == peer.FingerprintID && self.IPPrefix != "" && self.IPPrefix == peer.IPPrefix {
		addEvidence(acc, "fingerprint_ip", "strong", 45, map[string]any{"fingerprintId": self.FingerprintID, "ipPrefix": self.IPPrefix})
	}
	if self.DeviceFamilyHash != "" && self.DeviceFamilyHash == peer.DeviceFamilyHash {
		addEvidence(acc, "device_family", "medium", 18, map[string]any{"deviceFamilyHash": self.DeviceFamilyHash})
	}
	if self.IPPrefix != "" && self.IPPrefix == peer.IPPrefix {
		score := 10
		if self.Timezone == peer.Timezone && self.Timezone != "" {
			score += 2
		}
		addEvidence(acc, "ip_prefix", "weak", score, map[string]any{"ipPrefix": self.IPPrefix})
	}
	if self.BrowserFamilyHash != "" && self.BrowserFamilyHash == peer.BrowserFamilyHash {
		addEvidence(acc, "browser_family", "weak", 5, map[string]any{"browserFamilyHash": self.BrowserFamilyHash})
	}
	if self.Timezone != "" && self.Timezone == peer.Timezone && self.Language != "" && self.Language == peer.Language {
		addEvidence(acc, "locale_family", "weak", 4, map[string]any{"timezone": self.Timezone, "language": self.Language})
	}
}

func addEvidence(acc *evidenceAccumulator, key, strength string, score int, payload map[string]any) {
	item, exists := acc.items[key]
	if exists && item.score >= score {
		return
	}
	if exists {
		acc.totalScore -= item.score
	}
	acc.items[key] = evidenceItem{strength: strength, score: score, payload: payload}
	acc.totalScore += score
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func accumulateBehaviorEvidence(acc *evidenceAccumulator, selfSignals, peerSignals []signalRecord) {
	if len(selfSignals) < 5 || len(peerSignals) < 5 {
		return
	}

	// Session handoff: accounts appear within 5 minutes of each other 3+ times
	handoffCount := 0
	for _, self := range selfSignals {
		for _, peer := range peerSignals {
			diff := self.OccurredAt.Sub(peer.OccurredAt)
			if diff < 0 {
				diff = -diff
			}
			if diff < 5*time.Minute {
				handoffCount++
			}
		}
	}
	if handoffCount >= 3 {
		score := minInt(25+handoffCount*2, 40)
		addEvidence(acc, "session_handoff", "strong", score, map[string]any{"handoffCount": handoffCount})
	}

	// Never concurrent: accounts with ≥5 signals never appear within 2h of each other
	hasConcurrent := false
	for _, self := range selfSignals {
		for _, peer := range peerSignals {
			diff := self.OccurredAt.Sub(peer.OccurredAt)
			if diff < 0 {
				diff = -diff
			}
			if diff < 2*time.Hour {
				hasConcurrent = true
				break
			}
		}
		if hasConcurrent {
			break
		}
	}
	if !hasConcurrent {
		addEvidence(acc, "never_concurrent", "medium", 15, map[string]any{
			"selfCount": len(selfSignals),
			"peerCount": len(peerSignals),
		})
	}

	// Login pattern: cosine similarity of hourly login distribution ≥ 0.85
	if len(selfSignals) >= 10 && len(peerSignals) >= 10 {
		var selfHours [24]float64
		var peerHours [24]float64
		for _, s := range selfSignals {
			selfHours[s.OccurredAt.Hour()]++
		}
		for _, p := range peerSignals {
			peerHours[p.OccurredAt.Hour()]++
		}
		var dot, selfMag, peerMag float64
		for i := 0; i < 24; i++ {
			dot += selfHours[i] * peerHours[i]
			selfMag += selfHours[i] * selfHours[i]
			peerMag += peerHours[i] * peerHours[i]
		}
		if selfMag > 0 && peerMag > 0 {
			similarity := dot / (math.Sqrt(selfMag) * math.Sqrt(peerMag))
			if similarity >= 0.85 {
				addEvidence(acc, "login_pattern", "medium", 12, map[string]any{
					"cosineSimilarity": fmt.Sprintf("%.2f", similarity),
				})
			}
		}
	}
}

func (s *Service) loadPasswordPeers(ctx context.Context, tx pgx.Tx, userID int64) ([]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT other.user_id
		FROM risk_password_evidence mine
		JOIN risk_password_evidence other ON other.password_hash_digest = mine.password_hash_digest
		WHERE mine.user_id = $1
		  AND other.user_id <> $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, 4)
	for rows.Next() {
		var peerUserID int64
		if err := rows.Scan(&peerUserID); err != nil {
			return nil, err
		}
		out = append(out, peerUserID)
	}
	return out, rows.Err()
}

func (s *Service) loadTransferSummaries(ctx context.Context, tx pgx.Tx, userID int64) (map[int64]*transferSummary, error) {
	rows, err := tx.Query(ctx, `
		SELECT
			CASE WHEN from_user_id = $1 THEN to_user_id ELSE from_user_id END AS peer_user_id,
			channel,
			from_user_id,
			to_user_id,
			COUNT(*)::INT,
			COALESCE(SUM(net_amount), 0),
			COALESCE(MAX(net_amount), 0),
			COALESCE(SUM(CASE
				WHEN price_anchor > 0 AND ((unit_price::numeric / GREATEST(price_anchor, 1)::numeric) < 0.4
				  OR (unit_price::numeric / GREATEST(price_anchor, 1)::numeric) > 2.5)
				THEN 1 ELSE 0 END), 0)::INT,
			MIN(occurred_at),
			MAX(occurred_at)
		FROM risk_account_transfers
		WHERE (from_user_id = $1 OR to_user_id = $1)
		  AND occurred_at >= now() - interval '90 days'
		GROUP BY CASE WHEN from_user_id = $1 THEN to_user_id ELSE from_user_id END, channel, from_user_id, to_user_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]*transferSummary)
	for rows.Next() {
		var peerUserID int64
		var channel string
		var fromUserID int64
		var toUserID int64
		var count int
		var netAmount int64
		var maxAmount int64
		var abnormalCount int
		var firstSeenAt time.Time
		var lastSeenAt time.Time
		if err := rows.Scan(&peerUserID, &channel, &fromUserID, &toUserID, &count, &netAmount, &maxAmount, &abnormalCount, &firstSeenAt, &lastSeenAt); err != nil {
			return nil, err
		}
		summary := out[peerUserID]
		if summary == nil {
			summary = &transferSummary{
				peerUserID: peerUserID,
				channels:   make(map[string]int),
			}
			out[peerUserID] = summary
		}
		summary.totalCount += count
		summary.abnormalCount += abnormalCount
		summary.channels[channel] += count
		isMarketOrAuction := strings.HasPrefix(channel, "market_") || strings.HasPrefix(channel, "auction_")
		if strings.HasPrefix(channel, "market_") {
			summary.marketCount += count
		}
		if strings.HasPrefix(channel, "auction_") {
			summary.auctionCount += count
		}
		if fromUserID == userID {
			summary.outCount += count
			summary.grossOut += netAmount
			if !isMarketOrAuction {
				summary.directOutCount += count
			}
		} else if toUserID == userID {
			summary.inCount += count
			summary.grossIn += netAmount
			if !isMarketOrAuction {
				summary.directInCount += count
			}
		}
		if maxAmount > summary.maxSingle {
			summary.maxSingle = maxAmount
		}
		if summary.firstSeenAt.IsZero() || firstSeenAt.Before(summary.firstSeenAt) {
			summary.firstSeenAt = firstSeenAt
		}
		if lastSeenAt.After(summary.lastSeenAt) {
			summary.lastSeenAt = lastSeenAt
		}
	}
	return out, rows.Err()
}

func (s *Service) replaceUserEvidence(ctx context.Context, tx pgx.Tx, userID int64, evidenceMap map[int64]*evidenceAccumulator) error {
	if _, err := tx.Exec(ctx, `DELETE FROM risk_account_evidence WHERE user_id = $1 OR peer_user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM risk_account_links WHERE user_id = $1 OR peer_user_id = $1`, userID); err != nil {
		return err
	}
	for peerUserID, acc := range evidenceMap {
		level := riskLevelForScore(acc.totalScore)
		summary := make(map[string]any, len(acc.items))
		for key, item := range acc.items {
			payloadJSON, err := json.Marshal(item.payload)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO risk_account_evidence(
					user_id, peer_user_id, evidence_type, strength, score, first_seen_at, last_seen_at, evidence_payload_jsonb
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
			`, userID, peerUserID, key, item.strength, item.score, acc.minSeenAt, acc.maxSeenAt, string(payloadJSON)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO risk_account_evidence(
					user_id, peer_user_id, evidence_type, strength, score, first_seen_at, last_seen_at, evidence_payload_jsonb
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
			`, peerUserID, userID, key, item.strength, item.score, acc.minSeenAt, acc.maxSeenAt, string(payloadJSON)); err != nil {
				return err
			}
			summary[key] = map[string]any{"strength": item.strength, "score": item.score}
		}
		summaryJSON, err := json.Marshal(summary)
		if err != nil {
			return err
		}
		for _, pair := range [][2]int64{{userID, peerUserID}, {peerUserID, userID}} {
			if _, err := tx.Exec(ctx, `
				INSERT INTO risk_account_links(
					user_id, peer_user_id, total_score, risk_level, evidence_summary_jsonb, first_seen_at, last_seen_at, updated_at
				) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, now())
			`, pair[0], pair[1], acc.totalScore, level, string(summaryJSON), acc.minSeenAt, acc.maxSeenAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) collectClusterComponent(ctx context.Context, tx pgx.Tx, userID int64) ([]int64, error) {
	seen := map[int64]struct{}{userID: {}}
	queue := []int64{userID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		rows, err := tx.Query(ctx, `
			SELECT peer_user_id
			FROM risk_account_links
			WHERE user_id = $1
			  AND total_score >= 25
		`, current)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var peerUserID int64
			if err := rows.Scan(&peerUserID); err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := seen[peerUserID]; ok {
				continue
			}
			seen[peerUserID] = struct{}{}
			queue = append(queue, peerUserID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	out := make([]int64, 0, len(seen))
	for memberID := range seen {
		out = append(out, memberID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func (s *Service) upsertClusterState(ctx context.Context, tx pgx.Tx, memberIDs []int64) error {
	clusterID, err := s.resolveClusterID(ctx, tx, memberIDs)
	if err != nil {
		return err
	}
	memberScores := make(map[int64]int, len(memberIDs))
	maxScore := 0
	for _, memberID := range memberIDs {
		rows, err := tx.Query(ctx, `
			SELECT total_score
			FROM risk_account_links
			WHERE user_id = $1
			  AND peer_user_id = ANY($2)
		`, memberID, memberIDs)
		if err != nil {
			return err
		}
		memberMax := 0
		for rows.Next() {
			var score int
			if err := rows.Scan(&score); err != nil {
				rows.Close()
				return err
			}
			if score > memberMax {
				memberMax = score
			}
		}
		rows.Close()
		memberScores[memberID] = memberMax
		if memberMax > maxScore {
			maxScore = memberMax
		}
	}
	level := riskLevelForScore(maxScore)
	status := "active"
	if level == levelRestricted || level == levelSealed {
		status = "manual_review"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE risk_clusters
		SET risk_score = $2,
		    risk_level = $3,
		    status = $4,
		    member_count = $5,
		    primary_user_id = $6,
		    updated_at = now()
		WHERE cluster_id = $1
	`, clusterID, maxScore, level, status, len(memberIDs), memberIDs[0]); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM risk_cluster_members WHERE cluster_id = $1`, clusterID); err != nil {
		return err
	}
	for _, memberID := range memberIDs {
		reasonSummary := fmt.Sprintf("risk=%s score=%d", level, memberScores[memberID])
		if _, err := tx.Exec(ctx, `
			INSERT INTO risk_cluster_members(cluster_id, user_id, member_score, joined_at, reason_summary)
			VALUES($1, $2, $3, now(), $4)
		`, clusterID, memberID, memberScores[memberID], reasonSummary); err != nil {
			return err
		}
	}
	if err := s.replaceCapabilityBlocksTx(ctx, tx, clusterID, memberIDs, level, "risk_auto", "risk_auto"); err != nil {
		return err
	}
	if level == levelSealed {
		return s.insertReviewCaseTx(ctx, tx, clusterID, &memberIDs[0], "open", "sealed_candidate", "system", "auto created by risk service")
	}
	return nil
}

func (s *Service) resolveClusterID(ctx context.Context, tx pgx.Tx, memberIDs []int64) (int64, error) {
	var clusterID int64
	err := tx.QueryRow(ctx, `
		SELECT cluster_id
		FROM risk_cluster_members
		WHERE user_id = ANY($1)
		ORDER BY cluster_id ASC
		LIMIT 1
	`, memberIDs).Scan(&clusterID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `
			INSERT INTO risk_clusters(risk_score, risk_level, status, member_count, primary_user_id, updated_at)
			VALUES(0, 'observe', 'active', 0, NULL, now())
			RETURNING cluster_id
		`).Scan(&clusterID); err != nil {
			return 0, err
		}
		return clusterID, nil
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM risk_cluster_members WHERE user_id = ANY($1) AND cluster_id <> $2`, memberIDs, clusterID); err != nil {
		return 0, err
	}
	return clusterID, nil
}

func (s *Service) replaceCapabilityBlocksTx(ctx context.Context, tx pgx.Tx, clusterID int64, memberIDs []int64, level, reasonCode, source string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM risk_capability_blocks WHERE cluster_id = $1`, clusterID); err != nil {
		return err
	}
	if level != levelRestricted && level != levelSealed && level != levelBanned {
		return nil
	}
	capabilities := []string{CapabilityTrade, CapabilityTradeTransfer, CapabilityTeamJoin, CapabilityTeamInvite}
	for _, userID := range memberIDs {
		for _, capability := range capabilities {
			if _, err := tx.Exec(ctx, `
				INSERT INTO risk_capability_blocks(
					user_id, capability, block_level, reason_code, source, cluster_id, expires_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, NULL, now())
			`, userID, capability, level, reasonCode, source, clusterID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) insertReviewCaseTx(ctx context.Context, tx pgx.Tx, clusterID int64, userID *int64, status, decision, operatorName, operatorNote string) error {
	var existing int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM risk_review_cases
		WHERE cluster_id = $1
		  AND status = 'open'
		ORDER BY id DESC
		LIMIT 1
	`, clusterID).Scan(&existing)
	if err == nil {
		if _, err := tx.Exec(ctx, `
			UPDATE risk_review_cases
			SET updated_at = now(),
			    operator_name = $2,
			    operator_note = $3
			WHERE id = $1
		`, existing, strings.TrimSpace(operatorName), strings.TrimSpace(operatorNote)); err != nil {
			return err
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO risk_review_cases(cluster_id, user_id, status, decision, operator_name, operator_note, created_at, updated_at)
		VALUES($1, $2, $3, $4, $5, $6, now(), now())
	`, clusterID, userID, strings.TrimSpace(status), strings.TrimSpace(decision), strings.TrimSpace(operatorName), strings.TrimSpace(operatorNote))
	return err
}

func (s *Service) updateReviewCase(ctx context.Context, clusterID int64, operatorName, operatorNote, status, decision string) error {
	_, err := s.store.Pool().Exec(ctx, `
		UPDATE risk_review_cases
		SET status = $2,
		    decision = $3,
		    operator_name = $4,
		    operator_note = $5,
		    updated_at = now()
		WHERE cluster_id = $1
		  AND status = 'open'
	`, clusterID, status, decision, strings.TrimSpace(operatorName), strings.TrimSpace(operatorNote))
	return err
}

func (s *Service) listBlocks(ctx context.Context, userID int64) ([]CapabilityBlock, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT capability, block_level, reason_code, source, cluster_id, expires_at, updated_at
		FROM risk_capability_blocks
		WHERE user_id = $1
		  AND (expires_at IS NULL OR expires_at > now())
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CapabilityBlock, 0, 4)
	for rows.Next() {
		var item CapabilityBlock
		if err := rows.Scan(&item.Capability, &item.BlockLevel, &item.ReasonCode, &item.Source, &item.ClusterID, &item.ExpiresAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listSignals(ctx context.Context, userID int64, limit int) ([]TimelineRow, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT event_type, occurred_at, ip, ip_prefix, fingerprint_id, local_device_id, device_family_hash, browser_family_hash, timezone, accept_language, extra_jsonb
		FROM risk_account_signals
		WHERE user_id = $1
		ORDER BY occurred_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TimelineRow, 0, limit)
	for rows.Next() {
		var item TimelineRow
		var extraRaw []byte
		if err := rows.Scan(&item.EventType, &item.OccurredAt, &item.IP, &item.IPPrefix, &item.FingerprintID, &item.LocalDeviceID, &item.DeviceFamilyHash, &item.BrowserFamilyHash, &item.Timezone, &item.Language, &extraRaw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(extraRaw, &item.Extra)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listTransfers(ctx context.Context, userID int64, limit int) ([]TransferRow, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT
			CASE WHEN t.from_user_id = $1 THEN t.to_user_id ELSE t.from_user_id END AS peer_user_id,
			COALESCE(u.username, ''),
			CASE WHEN t.from_user_id = $1 THEN 'out' ELSE 'in' END AS direction,
			t.channel,
			t.reference_id,
			t.item_id,
			t.quantity,
			t.unit_price,
			t.total_price,
			t.fee_amount,
			t.net_amount,
			t.price_anchor,
			CASE WHEN t.price_anchor > 0 THEN ROUND(t.unit_price::numeric / GREATEST(t.price_anchor, 1)::numeric, 4)::float8 ELSE 0 END,
			t.occurred_at,
			t.extra_jsonb
		FROM risk_account_transfers t
		JOIN users u ON u.id = CASE WHEN t.from_user_id = $1 THEN t.to_user_id ELSE t.from_user_id END
		WHERE t.from_user_id = $1 OR t.to_user_id = $1
		ORDER BY t.occurred_at DESC, t.id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TransferRow, 0, limit)
	for rows.Next() {
		var item TransferRow
		var raw []byte
		if err := rows.Scan(
			&item.Peer.UserID,
			&item.Peer.Username,
			&item.Direction,
			&item.Channel,
			&item.ReferenceID,
			&item.ItemID,
			&item.Quantity,
			&item.UnitPrice,
			&item.TotalPrice,
			&item.FeeAmount,
			&item.NetAmount,
			&item.PriceAnchor,
			&item.PriceRatio,
			&item.OccurredAt,
			&raw,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &item.Extra)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listTransferPeers(ctx context.Context, userID int64, limit int) ([]TransferPeerSummary, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT
			CASE WHEN from_user_id = $1 THEN to_user_id ELSE from_user_id END AS peer_user_id,
			COALESCE(u.username, ''),
			COUNT(*)::INT,
			COALESCE(SUM(CASE WHEN channel LIKE 'market_%' THEN 1 ELSE 0 END), 0)::INT,
			COALESCE(SUM(CASE WHEN channel LIKE 'auction_%' THEN 1 ELSE 0 END), 0)::INT,
			COALESCE(SUM(CASE WHEN from_user_id = $1 THEN 1 ELSE 0 END), 0)::INT,
			COALESCE(SUM(CASE WHEN to_user_id = $1 THEN 1 ELSE 0 END), 0)::INT,
			COALESCE(SUM(CASE WHEN from_user_id = $1 THEN net_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN to_user_id = $1 THEN net_amount ELSE 0 END), 0),
			COALESCE(MAX(net_amount), 0),
			MAX(occurred_at)
		FROM risk_account_transfers t
		JOIN users u ON u.id = CASE WHEN from_user_id = $1 THEN to_user_id ELSE from_user_id END
		WHERE from_user_id = $1 OR to_user_id = $1
		GROUP BY CASE WHEN from_user_id = $1 THEN to_user_id ELSE from_user_id END, u.username
		ORDER BY COUNT(*) DESC, MAX(occurred_at) DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TransferPeerSummary, 0, limit)
	for rows.Next() {
		var item TransferPeerSummary
		if err := rows.Scan(
			&item.Peer.UserID,
			&item.Peer.Username,
			&item.TradeCount,
			&item.MarketCount,
			&item.AuctionCount,
			&item.OutgoingCount,
			&item.IncomingCount,
			&item.OutgoingNetAmount,
			&item.IncomingNetAmount,
			&item.MaxSingleNet,
			&item.LastOccurredAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listEvidence(ctx context.Context, userID int64, limit int) ([]EvidenceRow, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT e.peer_user_id, u.username, e.evidence_type, e.strength, e.score, e.first_seen_at, e.last_seen_at, e.evidence_payload_jsonb
		FROM risk_account_evidence e
		JOIN users u ON u.id = e.peer_user_id
		WHERE e.user_id = $1
		ORDER BY e.score DESC, e.last_seen_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EvidenceRow, 0, limit)
	for rows.Next() {
		var item EvidenceRow
		var raw []byte
		if err := rows.Scan(&item.Peer.UserID, &item.Peer.Username, &item.EvidenceType, &item.Strength, &item.Score, &item.FirstSeenAt, &item.LastSeenAt, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &item.Payload)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listLinkedUsers(ctx context.Context, userID int64, limit int) ([]LinkedUser, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT l.peer_user_id, u.username, l.total_score, l.risk_level, l.first_seen_at, l.last_seen_at
		FROM risk_account_links l
		JOIN users u ON u.id = l.peer_user_id
		WHERE l.user_id = $1
		ORDER BY l.total_score DESC, l.last_seen_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LinkedUser, 0, limit)
	for rows.Next() {
		var item LinkedUser
		if err := rows.Scan(&item.Peer.UserID, &item.Peer.Username, &item.TotalScore, &item.RiskLevel, &item.FirstSeenAt, &item.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listReviewCasesForUser(ctx context.Context, userID int64) ([]ReviewCase, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT id, cluster_id, user_id, status, decision, operator_name, operator_note, created_at, updated_at
		FROM risk_review_cases
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ReviewCase, 0, 8)
	for rows.Next() {
		var item ReviewCase
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.UserID, &item.Status, &item.Decision, &item.OperatorName, &item.OperatorNote, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listReviewCasesForCluster(ctx context.Context, clusterID int64) ([]ReviewCase, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT id, cluster_id, user_id, status, decision, operator_name, operator_note, created_at, updated_at
		FROM risk_review_cases
		WHERE cluster_id = $1
		ORDER BY updated_at DESC
		LIMIT 20
	`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ReviewCase, 0, 8)
	for rows.Next() {
		var item ReviewCase
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.UserID, &item.Status, &item.Decision, &item.OperatorName, &item.OperatorNote, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Service) listClusterMemberIDs(ctx context.Context, clusterID int64) ([]int64, error) {
	rows, err := s.store.Pool().Query(ctx, `
		SELECT user_id
		FROM risk_cluster_members
		WHERE cluster_id = $1
		ORDER BY user_id ASC
	`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, 8)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		out = append(out, userID)
	}
	if len(out) == 0 {
		return nil, ErrClusterNotFound
	}
	return out, rows.Err()
}

func (s *Service) AdminResetAllRiskData(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.WithTx(ctx, func(tx pgx.Tx) error {
		for _, table := range []string{
			"risk_capability_blocks",
			"risk_cluster_members",
			"risk_clusters",
			"risk_review_cases",
			"risk_account_evidence",
			"risk_account_links",
			"risk_account_transfers",
			"risk_account_signals",
			"risk_password_evidence",
		} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+table); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) AdminListAuctionTransfers(ctx context.Context, limit, offset int) (PaginatedResult[AuctionTransferItem], error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT COUNT(1) FROM risk_account_transfers WHERE channel LIKE 'auction_%'
	`).Scan(&total); err != nil {
		return PaginatedResult[AuctionTransferItem]{}, err
	}
	rows, err := s.store.Pool().Query(ctx, `
		SELECT t.id,
		       t.from_user_id, COALESCE(fu.username, ''),
		       t.to_user_id,   COALESCE(tu.username, ''),
		       t.channel, t.reference_id, t.item_id, t.quantity,
		       t.unit_price, t.total_price, t.fee_amount, t.net_amount, t.price_anchor,
		       CASE WHEN t.price_anchor > 0
		            THEN ROUND(t.unit_price::numeric / GREATEST(t.price_anchor,1)::numeric, 4)::float8
		            ELSE 0 END,
		       t.occurred_at, t.extra_jsonb
		FROM risk_account_transfers t
		JOIN users fu ON fu.id = t.from_user_id
		JOIN users tu ON tu.id = t.to_user_id
		WHERE t.channel LIKE 'auction_%'
		ORDER BY t.occurred_at DESC, t.id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return PaginatedResult[AuctionTransferItem]{}, err
	}
	defer rows.Close()
	out := make([]AuctionTransferItem, 0, limit)
	for rows.Next() {
		var item AuctionTransferItem
		var raw []byte
		if err := rows.Scan(
			&item.ID,
			&item.FromUser.UserID, &item.FromUser.Username,
			&item.ToUser.UserID, &item.ToUser.Username,
			&item.Channel, &item.ReferenceID, &item.ItemID, &item.Quantity,
			&item.UnitPrice, &item.TotalPrice, &item.FeeAmount, &item.NetAmount, &item.PriceAnchor,
			&item.PriceRatio,
			&item.OccurredAt, &raw,
		); err != nil {
			return PaginatedResult[AuctionTransferItem]{}, err
		}
		_ = json.Unmarshal(raw, &item.Extra)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return PaginatedResult[AuctionTransferItem]{}, err
	}
	return PaginatedResult[AuctionTransferItem]{Items: out, Total: total, Limit: limit, Offset: offset}, nil
}

func poolOf(st *store.Store) *pgxpool.Pool {
	if st == nil {
		return nil
	}
	return st.Pool()
}

func (s *Service) AdminListPasswordGroups(ctx context.Context, limit, offset int) (PaginatedResult[PasswordGroup], error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := s.store.Pool().QueryRow(ctx, `
		SELECT COUNT(1) FROM (
			SELECT password_hash_digest
			FROM risk_password_evidence
			GROUP BY password_hash_digest
			HAVING count(*) >= 2
		) sub
	`).Scan(&total); err != nil {
		return PaginatedResult[PasswordGroup]{}, err
	}
	rows, err := s.store.Pool().Query(ctx, `
		SELECT rpe.password_hash_digest, array_agg(rpe.user_id ORDER BY rpe.user_id) AS user_ids
		FROM risk_password_evidence rpe
		GROUP BY rpe.password_hash_digest
		HAVING count(*) >= 2
		ORDER BY count(*) DESC, rpe.password_hash_digest
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return PaginatedResult[PasswordGroup]{}, err
	}
	defer rows.Close()
	out := make([]PasswordGroup, 0, limit)
	for rows.Next() {
		var digest string
		var userIDs []int64
		if err := rows.Scan(&digest, &userIDs); err != nil {
			return PaginatedResult[PasswordGroup]{}, err
		}
		group := PasswordGroup{
			HashDigestHint: digest[:min(8, len(digest))],
			Users:          make([]PasswordGroupMember, 0, len(userIDs)),
		}
		for _, uid := range userIDs {
			var username string
			riskLevel := levelObserve
			riskScore := 0
			isBanned := false
			_ = s.store.Pool().QueryRow(ctx, `
				SELECT COALESCE(u.username, ''),
				       COALESCE(TRIM(u.ban_reason), '') <> '',
				       COALESCE(rc.risk_level, 'observe'),
				       COALESCE(rc.risk_score, 0)
				FROM users u
				LEFT JOIN risk_cluster_members rcm ON rcm.user_id = u.id
				LEFT JOIN risk_clusters rc ON rc.cluster_id = rcm.cluster_id
				WHERE u.id = $1
				LIMIT 1
			`, uid).Scan(&username, &isBanned, &riskLevel, &riskScore)
			group.Users = append(group.Users, PasswordGroupMember{
				UserID:    uid,
				Username:  username,
				RiskLevel: riskLevel,
				RiskScore: riskScore,
				IsBanned:  isBanned,
			})
		}
		out = append(out, group)
	}
	if err := rows.Err(); err != nil {
		return PaginatedResult[PasswordGroup]{}, err
	}
	return PaginatedResult[PasswordGroup]{Items: out, Total: total, Limit: limit, Offset: offset}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
