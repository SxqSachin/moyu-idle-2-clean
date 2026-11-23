package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"mysecondwebgame/server/internal/actionruntime"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/reqmetrics"
	"mysecondwebgame/server/internal/store"
	"mysecondwebgame/server/internal/ws"
	"mysecondwebgame/server/internal/wsbus"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	*fiber.App
	deps                        app.Dependencies
	loginGuard                  *loginAttemptGuard
	wsAuthGuard                 *loginAttemptGuard
	wsNATS                      wsNATSClient
	wsSubject                   string
	wsNodeSubject               string
	wsSubscriber                *nats.Subscription
	wsNodeSubscriber            *nats.Subscription
	actionSettledSub            *nats.Subscription
	wsTargetedRouting           bool
	wsNodeAdvertise             string
	actionWorkerStop            chan struct{}
	presenceReconcileStop       chan struct{}
	geetestBypassStop           chan struct{}
	eventDedupMu                sync.Mutex
	eventDedup                  map[string]eventDedupEntry
	eventDedupLastSweep         time.Time
	offlineFFMu                 sync.Mutex
	offlineFFReady              map[int64]offlineFastForwardState
	presenceMu                  sync.RWMutex
	presenceByUserID            map[int64]string
	presenceUpdatedAt           map[int64]time.Time
	presenceCursor              int64
	battleStreamMu              sync.RWMutex
	battleStreamSubs            map[string]struct{}
	arenaBattleStreamSubs       map[string]struct{}
	combatTrialBattleStreamSubs map[string]struct{}
	battleTickMu                sync.Mutex
	battleStopCh                chan struct{}
	battleLifecycleMu           sync.Mutex
	battleStopping              bool
	battleWorkerWG              sync.WaitGroup
	battleRunHistoryCleanup     *battleRunHistoryCleanupQueue
	battleStateMu               sync.RWMutex
	battleStateMem              map[int64]battleRuntimeState
	battlePlaybackMu            sync.RWMutex
	battlePlaybackMem           map[int64]battleRunPlaybackCache
	battlePlaybackTouch         map[int64]time.Time
	battlePlaybackSweep         time.Time
	battlePlaybackRunMu         sync.Mutex
	battlePlaybackRuns          map[int64]battlePlaybackWorkerState
	battlePlaybackRunSeq        uint64
	battleGapMu                 sync.Mutex
	battleLastFinishMs          map[int64]int64
	battleNextReadyMs           map[int64]int64
	battleCompileSlots          chan struct{}
	battleCompileFirstQ         chan battleCompileKickoffTask
	battleCompileQueue          chan battleCompileKickoffTask
	battleCompileRunMu          sync.Mutex
	battleCompileRuns           map[int64]struct{}
	battleCompileRetryMu        sync.Mutex
	battleCompileRetries        map[int64]battleCompileRetryState
	battleFinalizeSlots         chan struct{}
	battleAttrExpLogMu          sync.Mutex
	battleAttrExpLastLog        time.Time
	battleCheckpointMu          sync.RWMutex
	battleCheckpointMem         map[int64]battlePlaybackCheckpoint
	battleRoomDetailMu          sync.RWMutex
	battleRoomDetailMem         map[int64]battleRoomDetailCacheEntry
	playerProfileMu             sync.RWMutex
	playerProfileMem            map[int64]playerProfileCacheEntry
	playerProfileEpoch          map[int64]uint64
	playerCollectiblesMu        sync.RWMutex
	playerCollectiblesMem       map[int64]playerProfileCacheEntry
	playerAchievementsMu        sync.RWMutex
	playerAchievementsMem       map[int64]playerProfileCacheEntry
	playerPublicMu              sync.RWMutex
	playerPublicMem             map[int64]playerProfileCacheEntry
	playerPublicEpoch           map[int64]uint64
	battleDataMu                sync.RWMutex
	battleWorldCache            map[string]battleWorldConfig
	battleEnemyCache            map[string]battleEnemyConfig
	battleEnemyLoaded           bool
	// Fog mode pre-built pools (populated at server startup).
	fogNormalEnemyPool          []string
	fogSpecialEnemyPool         []string
	fogBaseEnemyLevel           int
	battleMetrics               battleMetricsCollector
	patchMu                     sync.Mutex
	inventoryPatchMu            [128]sync.Mutex
	inventoryPatchEpoch         map[int64]string
	patchEpoch                  string
	patchVersion                map[string]int64
	inventorySnap               map[int64]map[int64]string
	equipmentSnap               map[int64]map[int64]string
	attrSnap                    map[int64]attributePatchSnapshot
	roleEquipmentSnap           map[string]map[int64]string
	roleAttrSnap                map[string]attributePatchSnapshot
	patchUpdatedAt              map[int64]time.Time
	transientSweepMu            sync.Mutex
	transientLastSweep          time.Time
	publicRiskCooldownMu        sync.Mutex
	publicRiskCooldownUntil     map[int64]time.Time
	alchemyInheritCooldownMu    sync.Mutex
	alchemyInheritCooldownUntil map[int64]time.Time
	mutationSyncQueue           chan mutationSyncScope
	wsPressureMu                sync.Mutex
	wsPressure                  wsPressureCounters
	wsPressureLastSummary       time.Time
	wsSlowConsumerAt            []time.Time
	battleLoopEnabled           bool
	buffRulesOnce               sync.Once
	buffRulesCache              []battleBuffEffectRule
	buffRulesIndex              map[string][]battleBuffEffectRule
	buffMaxChargesCache         map[string]int64
	buffTickOnBattleStartCache  map[string]bool
	buffOnExpireCache           map[string]string
	buffKindCache               map[string]string
	buffDispellableCache        map[string]bool
	buffDurationTurnsCache      map[string]int64
	buffStackPolicyCache        map[string]string
	buffMaxStacksCache          map[string]int
}

const wsBattleBatchFlushTimeout = 500 * time.Millisecond

type wsNATSClient interface {
	IsConnected() bool
	MaxPayload() int64
	Publish(subject string, data []byte) error
	FlushTimeout(timeout time.Duration) error
}

type ServerOptions struct {
	EnableRoutes             bool
	SubscribeWSBroadcast     bool
	EnableActionQueueLoop    bool
	EnableBattleLoop         bool
	EnablePresenceReconciler bool
}

type battlePlaybackWorkerState struct {
	RoomID           int64
	RunID            int64
	Generation       uint64
	OwnerToken       string
	StartedAt        time.Time
	LastTickAt       time.Time
	LastProgressAt   time.Time
	LastNextTickAt   time.Time
	LastPushedSeq    int64
	LastNextEventIdx int
	RestartCount     int64
	StuckCount       int
}

type battleRoomDetailCacheEntry struct {
	Detail    store.BattleRoomDetail
	ExpiresAt time.Time
	CachedAt  time.Time
}

type playerProfileCacheEntry struct {
	Payload   fiber.Map
	ExpiresAt time.Time
	CachedAt  time.Time
}

const playerProfileCacheTTL = 60 * time.Second

const offlineFastForwardReadyTTL = 10 * time.Minute
const wsPresenceSessionTTL = 90 * time.Second
const wsTargetLookupMaxRetries = 3
const wsTargetLookupRetryDelay = 20 * time.Millisecond
const eventDedupEntryTTL = 10 * time.Minute
const eventDedupSweepInterval = 30 * time.Second
const eventDedupMaxEntries = 5000
const transientUserCacheTTL = 20 * time.Minute
const transientSweepInterval = 30 * time.Second
const transientMaxPrunePerSweep = 1024
const transientSweepSlowLogThreshold = 100 * time.Millisecond
const actionRuntimePendingSyncBatch = 32
const actionRuntimePendingSyncInterval = 2 * time.Second
const actionRuntimePendingSyncClaimTTL = 15 * time.Second

type eventDedupEntry struct {
	Payload   string
	UpdatedAt time.Time
}

type offlineFastForwardState struct {
	OfflineStartedAtMS     int64 `json:"offlineStartedAtMS"`
	OfflineTargetAtMS      int64 `json:"offlineTargetAtMS"`
	OfflineDurationSeconds int64 `json:"offlineDurationSeconds"`
	CreatedAtMS            int64 `json:"createdAtMS,omitempty"`
	markerRaw              string
}

var clearActionRuntimePendingSyncScript = redis.NewScript(`
local score = redis.call("ZSCORE", KEYS[1], ARGV[1])
if score == false then
	return 0
end
if tostring(score) == tostring(ARGV[2]) then
	redis.call("ZREM", KEYS[1], ARGV[1])
	redis.call("DEL", KEYS[2])
	redis.call("DEL", KEYS[3])
	return 1
end
return 0
`)

var claimActionRuntimePendingSyncScript = redis.NewScript(`
local score = redis.call("ZSCORE", KEYS[1], ARGV[1])
if score == false then
	return 0
end
local claimed = redis.call("SET", KEYS[2], ARGV[2], "NX", "PX", ARGV[3])
if claimed then
	return 1
end
return 0
`)

var consumeOfflineFastForwardIfUnchangedScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	redis.call("DEL", KEYS[1])
	return 1
end
return 0
`)

const (
	wsTargetTypeBroadcastAll = wsbus.TargetTypeBroadcastAll
	wsTargetTypeUsers        = wsbus.TargetTypeUsers
)

type ipRateLimiter struct {
	max    int
	window time.Duration
	prefix string
	redis  *redis.Client
}

type loginAttemptGuard struct {
	maxFailures   int
	lockDuration  time.Duration
	failureWindow time.Duration
	redis         *redis.Client
	prefix        string
}

func DefaultGatewayServerOptions() ServerOptions {
	return ServerOptions{
		EnableRoutes:             true,
		SubscribeWSBroadcast:     true,
		EnableActionQueueLoop:    false,
		EnableBattleLoop:         false,
		EnablePresenceReconciler: true,
	}
}

func DefaultBattleWorkerOptions() ServerOptions {
	return ServerOptions{
		EnableRoutes:             false,
		SubscribeWSBroadcast:     false,
		EnableActionQueueLoop:    false,
		EnableBattleLoop:         true,
		EnablePresenceReconciler: false,
	}
}

func NewServer(deps app.Dependencies) *Server {
	return NewServerWithOptions(deps, DefaultGatewayServerOptions())
}

func NewBattleWorker(deps app.Dependencies) *Server {
	return NewServerWithOptions(deps, DefaultBattleWorkerOptions())
}

func NewServerWithOptions(deps app.Dependencies, opts ServerOptions) *Server {
	if deps.Store != nil {
		deps.Store.SetBattleStateMachineV2(true)
	}
	proxyHeader := ""
	if deps.Config.RealIPTrustedCheck {
		proxyHeader = strings.TrimSpace(deps.Config.RealIPProxyHeader)
		if proxyHeader == "" {
			proxyHeader = "X-Forwarded-For"
		}
	}
	var app *fiber.App
	if opts.EnableRoutes {
		app = fiber.New(fiber.Config{
			ProxyHeader:             proxyHeader,
			EnableTrustedProxyCheck: deps.Config.RealIPTrustedCheck,
			TrustedProxies:          deps.Config.RealIPTrustedProxies,
			EnableIPValidation:      deps.Config.RealIPEnableValidation,
		})
	}
	loginAttemptCfg := deps.Config.GlobalSettings.RateLimit.LoginAttempt
	wsAuthAttemptCfg := deps.Config.GlobalSettings.RateLimit.WSAuthAttempt
	compileKickoffSlots := maxInt(1, deps.Config.GlobalSettings.Battle.CompileKickoffSlots)
	s := &Server{
		App:  app,
		deps: deps,
		loginGuard: newLoginAttemptGuard(
			loginAttemptCfg.MaxFailures,
			time.Duration(loginAttemptCfg.LockDurationSeconds)*time.Second,
			time.Duration(loginAttemptCfg.FailureWindowSeconds)*time.Second,
			deps.Redis,
			"login-attempt",
		),
		wsAuthGuard: newLoginAttemptGuard(
			wsAuthAttemptCfg.MaxFailures,
			time.Duration(wsAuthAttemptCfg.LockDurationSeconds)*time.Second,
			time.Duration(wsAuthAttemptCfg.FailureWindowSeconds)*time.Second,
			deps.Redis,
			"ws-auth-attempt",
		),
		actionWorkerStop:            make(chan struct{}),
		presenceReconcileStop:       make(chan struct{}),
		geetestBypassStop:           make(chan struct{}),
		eventDedup:                  make(map[string]eventDedupEntry),
		offlineFFReady:              make(map[int64]offlineFastForwardState),
		presenceByUserID:            make(map[int64]string),
		presenceUpdatedAt:           make(map[int64]time.Time),
		battleStreamSubs:            make(map[string]struct{}),
		arenaBattleStreamSubs:       make(map[string]struct{}),
		combatTrialBattleStreamSubs: make(map[string]struct{}),
		battleStopCh:                make(chan struct{}),
		battleStateMem:              make(map[int64]battleRuntimeState),
		battlePlaybackMem:           make(map[int64]battleRunPlaybackCache),
		battlePlaybackTouch:         make(map[int64]time.Time),
		battlePlaybackRuns:          make(map[int64]battlePlaybackWorkerState),
		battleLastFinishMs:          make(map[int64]int64),
		battleNextReadyMs:           make(map[int64]int64),
		battleCompileSlots:          make(chan struct{}, compileKickoffSlots),
		battleCompileFirstQ:         make(chan battleCompileKickoffTask, compileKickoffSlots),
		battleCompileQueue:          make(chan battleCompileKickoffTask, maxInt(128, compileKickoffSlots*64)),
		battleCompileRuns:           make(map[int64]struct{}),
		battleCompileRetries:        make(map[int64]battleCompileRetryState),
		battleFinalizeSlots:         make(chan struct{}, maxInt(1, deps.Config.GlobalSettings.Battle.FinalizeSlots)),
		battleCheckpointMem:         make(map[int64]battlePlaybackCheckpoint),
		battleRoomDetailMem:         make(map[int64]battleRoomDetailCacheEntry),
		playerProfileMem:            make(map[int64]playerProfileCacheEntry),
		playerProfileEpoch:          make(map[int64]uint64),
		playerCollectiblesMem:       make(map[int64]playerProfileCacheEntry),
		playerAchievementsMem:       make(map[int64]playerProfileCacheEntry),
		playerPublicMem:             make(map[int64]playerProfileCacheEntry),
		playerPublicEpoch:           make(map[int64]uint64),
		battleWorldCache:            make(map[string]battleWorldConfig),
		battleEnemyCache:            make(map[string]battleEnemyConfig),
		patchEpoch:                  newWSPatchEpoch(),
		patchVersion:                make(map[string]int64),
		inventorySnap:               make(map[int64]map[int64]string),
		equipmentSnap:               make(map[int64]map[int64]string),
		attrSnap:                    make(map[int64]attributePatchSnapshot),
		roleEquipmentSnap:           make(map[string]map[int64]string),
		roleAttrSnap:                make(map[string]attributePatchSnapshot),
		patchUpdatedAt:              make(map[int64]time.Time),
		publicRiskCooldownUntil:     make(map[int64]time.Time),
		alchemyInheritCooldownUntil: make(map[int64]time.Time),
		mutationSyncQueue:           make(chan mutationSyncScope, 1024),
		wsTargetedRouting:           deps.Config.WSTargetedRouting,
		wsNodeAdvertise:             strings.TrimSpace(deps.Config.NodeAdvertiseAddr),
		battleLoopEnabled:           opts.EnableBattleLoop,
	}
	if opts.EnableRoutes {
		s.routes()
		s.startGeetestBypassPoller()
		s.startOfflineRecoveryWorker()
	}
	s.initFogModePools()
	if err := s.initWSBroadcast(opts.SubscribeWSBroadcast); err != nil {
		panic(err)
	}
	if opts.EnableActionQueueLoop {
		s.startActionQueueWorker()
	}
	if opts.EnableBattleLoop {
		s.startBattleLoop()
	}
	s.startMutationSyncWorker()
	if opts.SubscribeWSBroadcast {
		s.startActionRuntimePendingSyncReconciler()
	}
	if opts.EnablePresenceReconciler {
		s.startPresenceReconciler()
	}
	return s
}

func actionQueueDueKey() string {
	return actionruntime.QueueDueKey()
}

func actionQueueClaimKey(userID int64) string {
	return actionruntime.ClaimKey(userID)
}

func offlineFastForwardReadyKey(userID int64) string {
	return actionruntime.OfflineFastForwardReadyKey(userID)
}

type wsBroadcastEvent = wsbus.Event

func (s *Server) initWSBroadcast(subscribe bool) error {
	if s.deps.Config.NATSURL == "" {
		return errors.New("NATS_URL is required for ws broadcast")
	}
	opts := []nats.Option{
		nats.Name("mysecondwebgame-ws-broadcast"),
		nats.ErrorHandler(func(conn *nats.Conn, sub *nats.Subscription, err error) {
			subject := ""
			if sub != nil {
				subject = sub.Subject
			}
			if errors.Is(err, nats.ErrSlowConsumer) || strings.Contains(strings.ToLower(err.Error()), "slow consumer") {
				s.recordWSSlowConsumer(time.Now(), subject, err)
				return
			}
			log.Printf("ws broadcast pressure node=%s event=nats_async_error subject=%s err=%v", s.wsNodeAdvertise, subject, err)
		}),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			log.Printf("ws broadcast pressure node=%s event=nats_disconnected err=%v", s.wsNodeAdvertise, err)
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			log.Printf("ws broadcast pressure node=%s event=nats_reconnected url=%s", s.wsNodeAdvertise, conn.ConnectedUrl())
		}),
		nats.ClosedHandler(func(conn *nats.Conn) {
			log.Printf("ws broadcast pressure node=%s event=nats_closed last_err=%v", s.wsNodeAdvertise, conn.LastError())
		}),
	}
	conn, err := nats.Connect(s.deps.Config.NATSURL, opts...)
	if err != nil {
		return fmt.Errorf("connect nats for ws broadcast failed: %w", err)
	}
	subject := wsbus.BroadcastSubject(s.deps.Config.MQTopicPrefix)
	s.wsSubject = subject
	var sub *nats.Subscription
	var nodeSub *nats.Subscription
	nodeSubject := ""
	if subscribe {
		sub, err = conn.Subscribe(subject, s.handleWSBroadcastMessage)
		if err != nil {
			conn.Close()
			return fmt.Errorf("subscribe ws broadcast topic failed: %w", err)
		}
		if s.wsTargetedRouting && s.wsNodeAdvertise != "" {
			nodeSubject = s.wsNodeSubjectFor(s.wsNodeAdvertise)
			nodeSub, err = conn.Subscribe(nodeSubject, s.handleWSBroadcastMessage)
			if err != nil {
				sub.Unsubscribe()
				conn.Close()
				return fmt.Errorf("subscribe ws node topic failed: %w", err)
			}
		}
		settledSubject := actionruntime.SettledSubject(s.deps.Config.MQTopicPrefix)
		s.actionSettledSub, err = conn.QueueSubscribe(settledSubject, "mysecondwebgame-action-runtime-settled", s.handleActionRuntimeSettledMessage)
		if err != nil {
			if nodeSub != nil {
				nodeSub.Unsubscribe()
			}
			sub.Unsubscribe()
			conn.Close()
			return fmt.Errorf("subscribe action runtime settled topic failed: %w", err)
		}
	}
	s.wsNATS = conn
	s.wsNodeSubject = nodeSubject
	s.wsSubscriber = sub
	s.wsNodeSubscriber = nodeSub
	log.Printf("ws broadcast connected subject=%s node_subject=%s nats=%s targeted=%t node=%s", subject, nodeSubject, s.deps.Config.NATSURL, s.wsTargetedRouting, s.wsNodeAdvertise)
	return nil
}

func (s *Server) handleActionRuntimeSettledMessage(msg *nats.Msg) {
	var evt actionruntime.SettledEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		log.Printf("action runtime settled decode failed err=%v", err)
		return
	}
	s.applyActionRuntimeSettledEvent(context.Background(), evt)
}

func (s *Server) applyActionRuntimeSettledEvent(ctx context.Context, evt actionruntime.SettledEvent) {
	if evt.UserID <= 0 {
		return
	}
	pendingID := strings.TrimSpace(evt.PendingID)
	if pendingID != "" {
		claimed, err := s.claimActionRuntimePendingSync(ctx, pendingID)
		if err != nil {
			log.Printf("action runtime settled claim failed pending_id=%s err=%v", pendingID, err)
			return
		}
		if !claimed {
			return
		}
	}
	equipmentChanged := evt.EquipmentChanged || actionruntime.EquipmentChanged(evt.Summary)
	s.syncActionRuntimeSettled(evt.UserID, evt.Summary, equipmentChanged)
	if pendingID != "" && evt.PendingAtMS > 0 {
		s.clearActionRuntimePendingSync(ctx, pendingID, evt.PendingAtMS)
	}
}

func (s *Server) syncActionRuntimeSettled(userID int64, summary store.ActionFastForwardSummary, equipmentChanged bool) {
	if userID <= 0 {
		return
	}
	// The running deadline is already committed. Push the queue snapshot before
	// the remaining read models so the client does not sit at zero.
	s.pushActionQueueSnapshot(userID)
	s.pushPlayerState(userID)
	s.pushWarehouseItems(userID)
	if equipmentChanged {
		s.pushEquipmentDomain(userID)
	}
	s.pushPlayerAttributes(userID)
	s.pushEnhanceAttemptEvents(userID, summary.EnhanceResults)
	s.pushWorkLoadoutHints(userID, summary)
	if summary.FishingChanged {
		s.pushFishingState(userID)
	}
	if summary.OperationEffectsChanged {
		s.pushOperationEffectsSnapshot(userID)
	}
	s.syncKittyActionAttributes(userID, summary.ActorSummaries)
}

func (s *Server) startActionRuntimePendingSyncReconciler() {
	if s.deps.Redis == nil {
		return
	}
	go func() {
		s.drainActionRuntimePendingSync(context.Background(), actionRuntimePendingSyncBatch)
		ticker := time.NewTicker(actionRuntimePendingSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.drainActionRuntimePendingSync(context.Background(), actionRuntimePendingSyncBatch)
			}
		}
	}()
}

func (s *Server) drainActionRuntimePendingSync(ctx context.Context, batch int64) {
	if s.deps.Redis == nil {
		return
	}
	if batch <= 0 {
		batch = actionRuntimePendingSyncBatch
	}
	nowMS := time.Now().UnixMilli()
	users, err := s.deps.Redis.ZRangeByScoreWithScores(ctx, actionruntime.PendingSyncUsersKey(), &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(nowMS, 10),
		Count: batch,
	}).Result()
	if err != nil {
		log.Printf("action runtime pending sync scan failed err=%v", err)
		return
	}
	for _, user := range users {
		pendingID := strings.TrimSpace(fmt.Sprint(user.Member))
		if pendingID == "" {
			_ = s.deps.Redis.ZRem(ctx, actionruntime.PendingSyncUsersKey(), user.Member).Err()
			continue
		}
		claimed, err := s.claimActionRuntimePendingSync(ctx, pendingID)
		if err != nil {
			log.Printf("action runtime pending sync claim failed pending_id=%s err=%v", pendingID, err)
			continue
		}
		if !claimed {
			continue
		}
		userID, summary, equipmentChanged, err := s.loadActionRuntimePendingSync(ctx, pendingID)
		if err != nil {
			log.Printf("action runtime pending sync load failed pending_id=%s err=%v", pendingID, err)
			continue
		}
		if userID <= 0 {
			_ = s.deps.Redis.ZRem(ctx, actionruntime.PendingSyncUsersKey(), user.Member).Err()
			continue
		}
		s.syncActionRuntimeSettled(userID, summary, equipmentChanged)
		s.clearActionRuntimePendingSync(ctx, pendingID, int64(user.Score))
	}
}

func (s *Server) loadActionRuntimePendingSync(ctx context.Context, pendingID string) (int64, store.ActionFastForwardSummary, bool, error) {
	var summary store.ActionFastForwardSummary
	pendingID = strings.TrimSpace(pendingID)
	if s.deps.Redis == nil || pendingID == "" {
		return 0, summary, false, nil
	}
	values, err := s.deps.Redis.HGetAll(ctx, actionruntime.PendingSyncInfoKey(pendingID)).Result()
	if err != nil {
		return 0, summary, false, err
	}
	userID, _ := strconv.ParseInt(strings.TrimSpace(values["user_id"]), 10, 64)
	if userID <= 0 {
		userID = pendingSyncUserIDFallback(pendingID)
	}
	if raw := strings.TrimSpace(values["summary"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &summary); err != nil {
			return userID, summary, false, err
		}
	}
	equipmentChanged, _ := strconv.ParseBool(strings.TrimSpace(values["equipment_changed"]))
	return userID, summary, equipmentChanged || actionruntime.EquipmentChanged(summary), nil
}

func pendingSyncUserIDFallback(pendingID string) int64 {
	prefix, _, _ := strings.Cut(strings.TrimSpace(pendingID), ":")
	userID, _ := strconv.ParseInt(prefix, 10, 64)
	return userID
}

func (s *Server) claimActionRuntimePendingSync(ctx context.Context, pendingID string) (bool, error) {
	if s.deps.Redis == nil {
		return false, nil
	}
	pendingID = strings.TrimSpace(pendingID)
	if pendingID == "" {
		return false, nil
	}
	result, err := claimActionRuntimePendingSyncScript.Run(
		ctx,
		s.deps.Redis,
		[]string{actionruntime.PendingSyncUsersKey(), actionruntime.PendingSyncClaimKey(pendingID)},
		pendingID,
		s.wsNodeAdvertise,
		strconv.FormatInt(actionRuntimePendingSyncClaimTTL.Milliseconds(), 10),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *Server) clearActionRuntimePendingSync(ctx context.Context, pendingID string, pendingAtMS int64) {
	pendingID = strings.TrimSpace(pendingID)
	if s.deps.Redis == nil || pendingID == "" || pendingAtMS <= 0 {
		return
	}
	_, err := clearActionRuntimePendingSyncScript.Run(
		ctx,
		s.deps.Redis,
		[]string{actionruntime.PendingSyncUsersKey(), actionruntime.PendingSyncInfoKey(pendingID), actionruntime.PendingSyncClaimKey(pendingID)},
		pendingID,
		strconv.FormatInt(pendingAtMS, 10),
	).Result()
	if err != nil {
		log.Printf("action runtime pending sync clear failed pending_id=%s err=%v", pendingID, err)
	}
}

func (s *Server) broadcastWSCluster(eventType string, payload any, ctxOpt ...context.Context) {
	s.publishWSBroadcast(eventType, 0, payload, ctxOpt...)
}

func (s *Server) pushUserEventCluster(userID int64, eventType string, payload any, ctxOpt ...context.Context) {
	s.publishWSTargetUsers(eventType, []int64{userID}, payload, "", "", ctxOpt...)
}

func (s *Server) publishWSBroadcast(eventType string, userID int64, payload any, ctxOpt ...context.Context) {
	if userID > 0 {
		s.publishWSTargetUsers(eventType, []int64{userID}, payload, "", "", ctxOpt...)
		return
	}
	s.publishWSBroadcastAll(eventType, payload, ctxOpt...)
}

func (s *Server) publishWSBroadcastAll(eventType string, payload any, ctxOpt ...context.Context) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return
	}
	evt := wsbus.NewBroadcastAll(eventType, rawPayload, time.Now().UnixMilli())
	evt.Category = classifyWSEventCategory(eventType, evt.TargetType, evt.Category)
	s.publishWSBroadcastEnvelope(s.wsSubject, evt, ctxOpt...)
}

func (s *Server) publishWSBroadcastGlobalTargeted(eventType string, userIDs []int64, payload any, excludeClientID string, ctxOpt ...context.Context) {
	targets := compactUserIDs(userIDs)
	if len(targets) == 0 {
		return
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return
	}
	evt := wsbus.NewTargetUsers(eventType, targets, rawPayload, time.Now().UnixMilli(), "", excludeClientID)
	evt.Category = classifyWSEventCategory(eventType, evt.TargetType, evt.Category)
	s.publishWSBroadcastEnvelope(s.wsSubject, evt, ctxOpt...)
}

func (s *Server) publishWSTargetUsers(eventType string, userIDs []int64, payload any, requestID string, excludeClientID string, ctxOpt ...context.Context) bool {
	targets := compactUserIDs(userIDs)
	if len(targets) == 0 {
		return true
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	evtBase := wsbus.NewTargetUsers(eventType, nil, rawPayload, time.Now().UnixMilli(), requestID, excludeClientID)
	evtBase.Category = classifyWSEventCategory(eventType, evtBase.TargetType, evtBase.Category)
	if !s.wsTargetedRouting || s.wsNodeAdvertise == "" || s.deps.Redis == nil || s.wsNodeSubject == "" {
		evt := evtBase
		evt.UserIDs = targets
		return s.publishWSBroadcastEnvelope(s.wsSubject, evt, ctxOpt...)
	}
	grouped := make(map[string][]int64)
	for _, uid := range targets {
		nodes := s.lookupUserRouteNodes(uid)
		if len(nodes) == 0 {
			s.recordWSTargetDrop(uid, eventType, "route_not_found")
			continue
		}
		for _, nodeAddr := range nodes {
			grouped[nodeAddr] = append(grouped[nodeAddr], uid)
		}
	}
	if len(grouped) == 0 {
		// No active route means there is no connected recipient to block on. A
		// reconnecting client restores the authoritative battle state over HTTP.
		return true
	}
	published := false
	for nodeAddr, group := range grouped {
		evt := evtBase
		evt.UserIDs = group
		if s.publishWSBroadcastEnvelopeBuffered(s.wsNodeSubjectFor(nodeAddr), evt, ctxOpt...) {
			published = true
		}
	}
	if !published {
		return false
	}
	if isBattleRunBatchWSEventType(evtBase.EventType) && !s.flushWSBattleBatch(evtBase, ctxOpt...) {
		return false
	}
	// Playback owns one shared cursor per run rather than one cursor per user.
	// Once any live route confirms the batch, do not retry it globally and risk
	// replaying state on recipients that already received it. Failed routes use
	// the existing HTTP resync path.
	return true
}

func (s *Server) publishWSBroadcastEnvelope(subject string, evt wsBroadcastEvent, ctxOpt ...context.Context) bool {
	if !s.publishWSBroadcastEnvelopeBuffered(subject, evt, ctxOpt...) {
		return false
	}
	if isBattleRunBatchWSEventType(evt.EventType) {
		return s.flushWSBattleBatch(evt, ctxOpt...)
	}
	return true
}

func (s *Server) publishWSBroadcastEnvelopeBuffered(subject string, evt wsBroadcastEvent, ctxOpt ...context.Context) bool {
	if strings.TrimSpace(subject) == "" {
		return false
	}
	if s.wsNATS == nil {
		s.deliverWSBroadcastEvent(evt)
		return true
	}
	if isBattleRunBatchWSEventType(evt.EventType) && !s.wsNATS.IsConnected() {
		if shouldLogWSBroadcastPublishDetail(evt, time.Now()) {
			log.Printf("publish ws battle batch delayed while nats disconnected subject=%s target=%s", subject, evt.TargetType)
		}
		return false
	}
	preparedEvent, body, compacted, err := prepareWSBroadcastEnvelopeForPublish(
		evt,
		s.wsNATS.MaxPayload(),
		s.deps.Config.WSBattleBatchCompression,
	)
	if err != nil {
		if shouldLogWSBroadcastPublishDetail(evt, time.Now()) {
			log.Printf("publish ws event rejected before nats subject=%s type=%s target=%s err=%v", subject, evt.EventType, evt.TargetType, err)
		}
		return false
	}
	if compacted {
		log.Printf("publish ws inventory event compacted to refresh request subject=%s target=%s users=%d", subject, evt.TargetType, len(evt.UserIDs))
	} else if isWSBroadcastCompressedFrame(body) && shouldLogWSBroadcastPublishDetail(evt, time.Now()) {
		log.Printf("publish ws battle batch compressed for nats subject=%s target=%s users=%d bytes=%d max=%d", subject, evt.TargetType, len(evt.UserIDs), len(body), s.wsNATS.MaxPayload())
	}
	evt = preparedEvent
	startAt := time.Now()
	err = s.wsNATS.Publish(subject, body)
	elapsed := time.Since(startAt)
	if collector := reqmetrics.FromContext(firstContext(ctxOpt...)); collector != nil {
		collector.AddNATS(elapsed)
	}
	if err != nil {
		if shouldLogWSBroadcastPublishDetail(evt, time.Now()) {
			log.Printf("publish ws event failed subject=%s type=%s target=%s err=%v", subject, evt.EventType, evt.TargetType, err)
		}
		return false
	}
	if elapsed >= wsPublishSlowThreshold && (evt.TargetType == wsTargetTypeUsers || isBattleRunBatchWSEventType(evt.EventType)) {
		log.Printf("ws broadcast pressure node=%s event=publish_slow subject=%s type=%s target=%s elapsed_ms=%d", s.wsNodeAdvertise, subject, evt.EventType, evt.TargetType, elapsed.Milliseconds())
	}
	return true
}

func (s *Server) flushWSBattleBatch(evt wsBroadcastEvent, ctxOpt ...context.Context) bool {
	if s.wsNATS == nil {
		return true
	}
	startAt := time.Now()
	err := s.wsNATS.FlushTimeout(wsBattleBatchFlushTimeout)
	elapsed := time.Since(startAt)
	if collector := reqmetrics.FromContext(firstContext(ctxOpt...)); collector != nil {
		collector.AddNATS(elapsed)
	}
	if err != nil {
		if shouldLogWSBroadcastPublishDetail(evt, time.Now()) {
			log.Printf("flush ws battle batch failed target=%s timeout_ms=%d elapsed_ms=%d err=%v", evt.TargetType, wsBattleBatchFlushTimeout.Milliseconds(), elapsed.Milliseconds(), err)
		}
		return false
	}
	if elapsed >= wsPublishSlowThreshold {
		log.Printf("ws broadcast pressure node=%s event=flush_slow type=%s target=%s elapsed_ms=%d", s.wsNodeAdvertise, evt.EventType, evt.TargetType, elapsed.Milliseconds())
	}
	return true
}

func shouldLogWSBroadcastPublishDetail(evt wsBroadcastEvent, now time.Time) bool {
	if !isBattleRunBatchWSEventType(evt.EventType) {
		return true
	}
	runID := battleRunIDFromWirePayload(evt.Payload)
	if runID <= 0 {
		return true
	}
	return shouldSampleBattlePlaybackFailureLog(runID, now)
}

func battleRunIDFromWirePayload(payload json.RawMessage) int64 {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return 0
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0
		}
		if key == "r" || key == "runId" {
			var runID int64
			if err := decoder.Decode(&runID); err != nil || runID <= 0 {
				return 0
			}
			return runID
		}
		var discarded json.RawMessage
		if err := decoder.Decode(&discarded); err != nil {
			return 0
		}
	}
	return 0
}

func firstContext(ctxOpt ...context.Context) context.Context {
	if len(ctxOpt) == 0 {
		return nil
	}
	for _, ctx := range ctxOpt {
		if ctx != nil {
			return ctx
		}
	}
	return nil
}

func (s *Server) handleWSBroadcastMessage(msg *nats.Msg) {
	evt, compressed, err := decodeWSBroadcastEnvelope(msg.Data)
	if err != nil {
		if compressed {
			log.Printf("ws compressed broadcast decode failed subject=%s bytes=%d err=%v", msg.Subject, len(msg.Data), err)
		}
		return
	}
	s.deliverWSBroadcastEvent(evt)
}

func (s *Server) deliverWSBroadcastEvent(evt wsBroadcastEvent) {
	startedAt := time.Now()
	eventType := strings.TrimSpace(evt.EventType)
	if eventType == "" {
		eventType = strings.TrimSpace(evt.Type)
	}
	if eventType == "" {
		return
	}
	if eventType == playerProfileChangedEventType {
		s.applyPlayerProfileChangedEvent(evt.Payload)
	}
	var payload any
	if err := json.Unmarshal(evt.Payload, &payload); err != nil {
		return
	}
	targetType := strings.TrimSpace(evt.TargetType)
	if isBattleRunBatchWSEventType(eventType) && !isStrictBattleRunBatchEnvelope(targetType, evt.UserIDs, payload) {
		return
	}
	if isBattleRoomUpdatedWSEventType(eventType) && !isStrictBattleRoomUpdatedEnvelope(targetType, evt.UserIDs, payload) {
		return
	}
	if targetType == "" && evt.UserID > 0 {
		targetType = wsTargetTypeUsers
		evt.UserIDs = append(evt.UserIDs, evt.UserID)
	}
	category := classifyWSEventCategory(eventType, targetType, evt.Category)
	switch targetType {
	case wsTargetTypeUsers:
		battleBatchEvent := isBattleRunBatchWSEventType(eventType)
		targets := compactUserIDs(evt.UserIDs)
		if len(targets) == 0 {
			return
		}
		allowedTargets := make(map[int64]struct{}, len(targets))
		skippedPresence := 0
		for _, uid := range targets {
			if !isUserStateSyncEventType(eventType) && !s.shouldDeliverWSByPresence(uid, category) {
				if battleBatchEvent {
					skippedPresence++
				}
				continue
			}
			allowedTargets[uid] = struct{}{}
		}
		if len(allowedTargets) == 0 {
			s.recordWSPressure(time.Now(), func(counters *wsPressureCounters) {
				counters.TargetedSkipped += int64(len(targets))
			})
			if battleBatchEvent {
				s.battleMetrics.AddWSBattleBatchDelivery(0, 0, skippedPresence, 0)
			}
			return
		}
		targetSet := make(map[int64]struct{}, len(targets))
		for uid := range allowedTargets {
			targetSet[uid] = struct{}{}
		}
		battleAttempts := 0
		battleDelivered := 0
		battleSkippedSubscribed := 0
		attempts := 0
		delivered := 0
		skipped := skippedPresence
		for _, subscriber := range s.deps.Hub.Clients() {
			if _, ok := targetSet[subscriber.UserID]; !ok {
				continue
			}
			if evt.ExcludeClientID != "" && evt.ExcludeClientID == subscriber.ID {
				continue
			}
			if !s.canDeliverWSClientEvent(subscriber) {
				skipped++
				continue
			}
			attempts++
			if battleBatchEvent {
				battleAttempts++
			}
			if battleBatchEvent && !s.isBattleStreamSubscribedForPayload(subscriber.ID, payload) {
				battleSkippedSubscribed++
				skipped++
				continue
			}
			_ = s.writeWSEvent(subscriber.Conn, eventType, payload)
			delivered++
			if battleBatchEvent {
				battleDelivered++
			}
			if eventType == "auth.kicked" {
				_ = subscriber.Conn.Close()
			}
		}
		s.recordWSPressure(time.Now(), func(counters *wsPressureCounters) {
			counters.TargetedAttempts += int64(attempts)
			counters.TargetedDelivered += int64(delivered)
			counters.TargetedSkipped += int64(skipped)
			counters.BattleSkippedSubscribed += int64(battleSkippedSubscribed)
		})
		if battleBatchEvent {
			s.battleMetrics.AddWSBattleBatchDelivery(battleAttempts, battleDelivered, skippedPresence, battleSkippedSubscribed)
		}
	default:
		battleBatchEvent := isBattleRunBatchWSEventType(eventType)
		battleAttempts := 0
		battleDelivered := 0
		battleSkippedPresence := 0
		battleSkippedSubscribed := 0
		attempts := 0
		delivered := 0
		skipped := 0
		for _, subscriber := range s.deps.Hub.Clients() {
			if !s.canDeliverWSClientEvent(subscriber) {
				skipped++
				continue
			}
			attempts++
			if battleBatchEvent {
				battleAttempts++
			}
			if battleBatchEvent && !s.isBattleStreamSubscribedForPayload(subscriber.ID, payload) {
				battleSkippedSubscribed++
				skipped++
				continue
			}
			if !isUserStateSyncEventType(eventType) && !s.shouldDeliverWSByPresence(subscriber.UserID, category) {
				if battleBatchEvent {
					battleSkippedPresence++
				}
				skipped++
				continue
			}
			_ = s.writeWSEvent(subscriber.Conn, eventType, payload)
			delivered++
			if battleBatchEvent {
				battleDelivered++
			}
		}
		s.recordWSPressure(time.Now(), func(counters *wsPressureCounters) {
			counters.BroadcastAttempts += int64(attempts)
			counters.BroadcastDelivered += int64(delivered)
			counters.BroadcastSkipped += int64(skipped)
			counters.BattleSkippedSubscribed += int64(battleSkippedSubscribed)
		})
		if battleBatchEvent {
			s.battleMetrics.AddWSBattleBatchDelivery(battleAttempts, battleDelivered, battleSkippedPresence, battleSkippedSubscribed)
		}
	}
	elapsed := time.Since(startedAt)
	if elapsed >= wsDeliverySlowThreshold && (targetType == wsTargetTypeUsers || isBattleRunBatchWSEventType(eventType) || isUserStateSyncEventType(eventType)) {
		log.Printf("ws broadcast pressure node=%s event=deliver_slow type=%s target=%s elapsed_ms=%d", s.wsNodeAdvertise, eventType, targetType, elapsed.Milliseconds())
	}
}

func (s *Server) canDeliverWSClientEvent(client *ws.Client) bool {
	if client == nil || client.Conn == nil {
		return false
	}
	if client.UserID <= 0 || client.SessionVersion <= 0 {
		return false
	}
	return true
}

func classifyWSEventCategory(eventType string, targetType string, existing string) string {
	normalizedExisting := strings.TrimSpace(strings.ToLower(existing))
	switch normalizedExisting {
	case wsbus.EventCategoryGlobal, wsbus.EventCategoryControl, wsbus.EventCategoryUserState:
		return normalizedExisting
	}
	if isMustDeliverWSEventType(eventType) {
		return wsbus.EventCategoryControl
	}
	if strings.EqualFold(strings.TrimSpace(targetType), wsTargetTypeBroadcastAll) {
		return wsbus.EventCategoryGlobal
	}
	return wsbus.EventCategoryUserState
}

func isMustDeliverWSEventType(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "auth.kicked", "auth.bound", "error":
		return true
	default:
		return false
	}
}

func (s *Server) shouldDeliverWSByPresence(userID int64, category string) bool {
	normalized := strings.TrimSpace(strings.ToLower(category))
	if normalized == wsbus.EventCategoryGlobal || normalized == wsbus.EventCategoryControl {
		return true
	}
	return !s.isUserOfflineForDelivery(userID)
}

func isUserStateSyncEventType(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "player.attributes.updated",
		"equipment.instances.updated",
		"equipment.stats.updated",
		"inventory.instances.updated",
		"kitty.attributes.updated",
		"kitty.equipment.updated",
		"game.state.patch":
		return true
	default:
		return false
	}
}

func normalizePresenceStatus(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	if normalized == "offline" {
		return "offline"
	}
	return "online"
}

func (s *Server) setUserPresenceCache(userID int64, status string) {
	if userID <= 0 {
		return
	}
	s.presenceMu.Lock()
	defer s.presenceMu.Unlock()
	s.presenceByUserID[userID] = normalizePresenceStatus(status)
	s.presenceUpdatedAt[userID] = time.Now()
}

func (s *Server) clearUserPresenceCache(userID int64) {
	if userID <= 0 {
		return
	}
	s.presenceMu.Lock()
	delete(s.presenceByUserID, userID)
	delete(s.presenceUpdatedAt, userID)
	s.presenceMu.Unlock()
}

func (s *Server) clearUserTransientCachesIfInactive(userID int64) {
	if userID <= 0 {
		return
	}
	if len(s.userSockets(userID)) > 0 {
		return
	}
	s.clearUserPresenceCache(userID)
	s.resetRealtimePatchState(userID)
}

func (s *Server) getUserPresenceCache(userID int64) (string, bool) {
	if userID <= 0 {
		return "", false
	}
	s.presenceMu.RLock()
	defer s.presenceMu.RUnlock()
	status, ok := s.presenceByUserID[userID]
	return status, ok
}

func (s *Server) sweepEventDedupLocked(now time.Time) {
	for key, entry := range s.eventDedup {
		if now.Sub(entry.UpdatedAt) > eventDedupEntryTTL {
			delete(s.eventDedup, key)
		}
	}
	if len(s.eventDedup) <= eventDedupMaxEntries {
		return
	}
	type dedupItem struct {
		key string
		at  time.Time
	}
	items := make([]dedupItem, 0, len(s.eventDedup))
	for key, entry := range s.eventDedup {
		items = append(items, dedupItem{key: key, at: entry.UpdatedAt})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].at.Before(items[j].at)
	})
	excess := len(items) - eventDedupMaxEntries
	for idx := 0; idx < excess; idx++ {
		delete(s.eventDedup, items[idx].key)
	}
}

func (s *Server) sweepPresenceAndPatchCaches(now time.Time) {
	startedAt := time.Now()
	s.pruneExpiredOfflineFastForwardFallback(now)
	staleUsers := make(map[int64]struct{})
	s.presenceMu.RLock()
	for userID, touchedAt := range s.presenceUpdatedAt {
		if now.Sub(touchedAt) > transientUserCacheTTL {
			staleUsers[userID] = struct{}{}
		}
	}
	s.presenceMu.RUnlock()

	s.patchMu.Lock()
	for userID, touchedAt := range s.patchUpdatedAt {
		if now.Sub(touchedAt) > transientUserCacheTTL {
			staleUsers[userID] = struct{}{}
		}
	}
	s.patchMu.Unlock()
	if len(staleUsers) == 0 {
		return
	}

	activeUsers := make(map[int64]struct{})
	for _, client := range s.deps.Hub.Clients() {
		if client == nil || client.UserID <= 0 {
			continue
		}
		activeUsers[client.UserID] = struct{}{}
	}

	pruneUsers := make([]int64, 0, len(staleUsers))
	for userID := range staleUsers {
		if _, online := activeUsers[userID]; online {
			continue
		}
		pruneUsers = append(pruneUsers, userID)
		if len(pruneUsers) >= transientMaxPrunePerSweep {
			break
		}
	}
	if len(pruneUsers) == 0 {
		return
	}

	s.patchMu.Lock()
	for _, userID := range pruneUsers {
		delete(s.inventorySnap, userID)
		delete(s.inventoryPatchEpoch, userID)
		delete(s.equipmentSnap, userID)
		delete(s.attrSnap, userID)
		delete(s.patchUpdatedAt, userID)
		prefix := fmt.Sprintf("%d:", userID)
		for key := range s.patchVersion {
			if strings.HasPrefix(key, prefix) {
				delete(s.patchVersion, key)
			}
		}
		for key := range s.roleEquipmentSnap {
			if strings.HasPrefix(key, prefix) {
				delete(s.roleEquipmentSnap, key)
			}
		}
		for key := range s.roleAttrSnap {
			if strings.HasPrefix(key, prefix) {
				delete(s.roleAttrSnap, key)
			}
		}
	}
	s.patchMu.Unlock()

	s.presenceMu.Lock()
	for _, userID := range pruneUsers {
		delete(s.presenceByUserID, userID)
		delete(s.presenceUpdatedAt, userID)
	}
	s.presenceMu.Unlock()
	if elapsed := time.Since(startedAt); elapsed >= transientSweepSlowLogThreshold {
		log.Printf("transient cache sweep slow elapsed=%s stale_candidates=%d active_users=%d pruned=%d", elapsed, len(staleUsers), len(activeUsers), len(pruneUsers))
	}
}

func (s *Server) maybeSweepPresenceAndPatchCaches(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	s.transientSweepMu.Lock()
	if now.Sub(s.transientLastSweep) < transientSweepInterval {
		s.transientSweepMu.Unlock()
		return
	}
	s.transientLastSweep = now
	s.transientSweepMu.Unlock()
	s.sweepPresenceAndPatchCaches(now)
}

func (s *Server) memoryMapSizeSnapshot() map[string]int {
	s.eventDedupMu.Lock()
	eventDedupSize := len(s.eventDedup)
	s.eventDedupMu.Unlock()

	s.presenceMu.RLock()
	presenceSize := len(s.presenceByUserID)
	s.presenceMu.RUnlock()

	s.battleStreamMu.RLock()
	battleStreamSubsSize := len(s.battleStreamSubs)
	arenaBattleStreamSubsSize := len(s.arenaBattleStreamSubs)
	s.battleStreamMu.RUnlock()

	s.battleStateMu.RLock()
	battleStateSize := len(s.battleStateMem)
	s.battleStateMu.RUnlock()

	s.battlePlaybackMu.RLock()
	battlePlaybackCacheSize := len(s.battlePlaybackMem)
	battlePlaybackTouchSize := len(s.battlePlaybackTouch)
	s.battlePlaybackMu.RUnlock()

	s.battlePlaybackRunMu.Lock()
	battlePlaybackRunSize := len(s.battlePlaybackRuns)
	s.battlePlaybackRunMu.Unlock()

	s.battleCheckpointMu.RLock()
	battleCheckpointSize := len(s.battleCheckpointMem)
	s.battleCheckpointMu.RUnlock()

	s.patchMu.Lock()
	patchVersionSize := len(s.patchVersion)
	inventorySnapSize := len(s.inventorySnap)
	equipmentSnapSize := len(s.equipmentSnap)
	attrSnapSize := len(s.attrSnap)
	roleEquipmentSnapSize := len(s.roleEquipmentSnap)
	roleAttrSnapSize := len(s.roleAttrSnap)
	patchUpdatedSize := len(s.patchUpdatedAt)
	s.patchMu.Unlock()

	s.offlineFFMu.Lock()
	offlineFFReadySize := len(s.offlineFFReady)
	s.offlineFFMu.Unlock()

	return map[string]int{
		"eventDedup":            eventDedupSize,
		"presenceByUserID":      presenceSize,
		"battleStreamSubs":      battleStreamSubsSize,
		"arenaBattleStreamSubs": arenaBattleStreamSubsSize,
		"battleStateMem":        battleStateSize,
		"battlePlaybackMem":     battlePlaybackCacheSize,
		"battlePlaybackTouch":   battlePlaybackTouchSize,
		"battlePlaybackRuns":    battlePlaybackRunSize,
		"battleCheckpointMem":   battleCheckpointSize,
		"patchVersion":          patchVersionSize,
		"inventorySnap":         inventorySnapSize,
		"equipmentSnap":         equipmentSnapSize,
		"attrSnap":              attrSnapSize,
		"roleEquipmentSnap":     roleEquipmentSnapSize,
		"roleAttrSnap":          roleAttrSnapSize,
		"patchSnapshotTouched":  patchUpdatedSize,
		"offlineFFReady":        offlineFFReadySize,
	}
}

func (s *Server) isUserOfflineForDelivery(userID int64) bool {
	if userID <= 0 {
		return false
	}
	if status, ok := s.getUserPresenceCache(userID); ok {
		return normalizePresenceStatus(status) == "offline"
	}
	presence, err := s.deps.Store.GetPresence(context.Background(), userID)
	if err != nil {
		return false
	}
	status := normalizePresenceStatus(presence.Status)
	s.setUserPresenceCache(userID, status)
	return status == "offline"
}

func isBattleRunBatchWSEventType(eventType string) bool {
	return strings.EqualFold(strings.TrimSpace(eventType), "battle.run.batch")
}

func isBattleRoomUpdatedWSEventType(eventType string) bool {
	return strings.EqualFold(strings.TrimSpace(eventType), "battle.room.updated")
}

const (
	battleStreamKindRoom        = "room"
	battleStreamKindArena       = "arena"
	battleStreamKindCombatTrial = "combat_trial"
)

func normalizeBattleStreamKind(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), battleStreamKindCombatTrial) {
		return battleStreamKindCombatTrial
	}
	if strings.EqualFold(strings.TrimSpace(raw), battleStreamKindArena) {
		return battleStreamKindArena
	}
	return battleStreamKindRoom
}

func battleEventStreamKind(payload any) string {
	var root map[string]any
	switch value := payload.(type) {
	case battleEventBatchWirePayload:
		return normalizeBattleStreamKind(value.K)
	case *battleEventBatchWirePayload:
		if value == nil {
			return battleStreamKindRoom
		}
		return normalizeBattleStreamKind(value.K)
	case map[string]any:
		root = value
	case fiber.Map:
		root = map[string]any(value)
	default:
		return battleStreamKindRoom
	}
	for _, key := range []string{"k", "battleKind"} {
		if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", root[key])), battleKindCombatTrial) {
			return battleStreamKindCombatTrial
		}
		if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", root[key])), battleKindArena) {
			return battleStreamKindArena
		}
	}
	return battleStreamKindRoom
}

func (s *Server) setBattleStreamSubscribed(clientID string, enabled bool) {
	s.setBattleStreamKindSubscribed(clientID, battleStreamKindRoom, enabled)
}

func (s *Server) setBattleStreamKindSubscribed(clientID, kind string, enabled bool) {
	normalizedClientID := strings.TrimSpace(clientID)
	if normalizedClientID == "" {
		return
	}
	kind = normalizeBattleStreamKind(kind)
	s.battleStreamMu.Lock()
	defer s.battleStreamMu.Unlock()
	target := s.battleStreamSubs
	if kind == battleStreamKindArena {
		target = s.arenaBattleStreamSubs
	} else if kind == battleStreamKindCombatTrial {
		target = s.combatTrialBattleStreamSubs
	}
	if target == nil {
		target = make(map[string]struct{})
		if kind == battleStreamKindArena {
			s.arenaBattleStreamSubs = target
		} else if kind == battleStreamKindCombatTrial {
			s.combatTrialBattleStreamSubs = target
		} else {
			s.battleStreamSubs = target
		}
	}
	if enabled {
		target[normalizedClientID] = struct{}{}
		return
	}
	delete(target, normalizedClientID)
}

func (s *Server) clearBattleStreamSubscribed(clientID string) {
	s.setBattleStreamKindSubscribed(clientID, battleStreamKindRoom, false)
	s.setBattleStreamKindSubscribed(clientID, battleStreamKindArena, false)
	s.setBattleStreamKindSubscribed(clientID, battleStreamKindCombatTrial, false)
}

func (s *Server) isBattleStreamSubscribed(clientID string) bool {
	return s.isBattleStreamKindSubscribed(clientID, battleStreamKindRoom)
}

func (s *Server) isBattleStreamKindSubscribed(clientID, kind string) bool {
	normalizedClientID := strings.TrimSpace(clientID)
	if normalizedClientID == "" {
		return false
	}
	kind = normalizeBattleStreamKind(kind)
	s.battleStreamMu.RLock()
	defer s.battleStreamMu.RUnlock()
	target := s.battleStreamSubs
	if kind == battleStreamKindArena {
		target = s.arenaBattleStreamSubs
	} else if kind == battleStreamKindCombatTrial {
		target = s.combatTrialBattleStreamSubs
	}
	_, ok := target[normalizedClientID]
	return ok
}

func (s *Server) isBattleStreamSubscribedForPayload(clientID string, payload any) bool {
	return s.isBattleStreamKindSubscribed(clientID, battleEventStreamKind(payload))
}

func isStrictBattleRunBatchEnvelope(targetType string, userIDs []int64, payload any) bool {
	if strings.TrimSpace(targetType) != wsTargetTypeUsers {
		return false
	}
	if len(compactUserIDs(userIDs)) == 0 {
		return false
	}
	roomID, ok := battleEventRoomID(payload)
	return ok && roomID > 0
}

func isStrictBattleRoomUpdatedEnvelope(targetType string, userIDs []int64, payload any) bool {
	if strings.TrimSpace(targetType) != wsTargetTypeUsers {
		return false
	}
	if len(compactUserIDs(userIDs)) == 0 {
		return false
	}
	roomID, ok := battleEventRoomID(payload)
	return ok && roomID > 0
}

func battleEventRoomID(payload any) (int64, bool) {
	root, ok := payload.(map[string]any)
	if !ok || len(root) == 0 {
		return 0, false
	}
	if roomID, ok := parseAnyInt64(root["roomId"]); ok && roomID > 0 {
		return roomID, true
	}
	if roomID, ok := parseAnyInt64(root["m"]); ok && roomID > 0 {
		return roomID, true
	}
	roomRaw, ok := root["room"]
	if !ok {
		return 0, false
	}
	roomMap, ok := roomRaw.(map[string]any)
	if !ok || len(roomMap) == 0 {
		return 0, false
	}
	if roomID, ok := parseAnyInt64(roomMap["id"]); ok && roomID > 0 {
		return roomID, true
	}
	return 0, false
}

func parseAnyInt64(raw any) (int64, bool) {
	switch typed := raw.(type) {
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case json.Number:
		value, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return value, true
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return 0, false
		}
		value, err := strconv.ParseInt(normalized, 10, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	default:
		return 0, false
	}
}

func (s *Server) userSockets(userID int64) []*websocket.Conn {
	sockets := make([]*websocket.Conn, 0, 2)
	for _, subscriber := range s.deps.Hub.Clients() {
		if subscriber.UserID != userID {
			continue
		}
		sockets = append(sockets, subscriber.Conn)
	}
	return sockets
}

func (s *Server) userSessionSockets(userID, sessionVersion int64) []*websocket.Conn {
	sockets := make([]*websocket.Conn, 0, 2)
	for _, subscriber := range s.deps.Hub.Clients() {
		if subscriber.UserID != userID || subscriber.SessionVersion != sessionVersion {
			continue
		}
		sockets = append(sockets, subscriber.Conn)
	}
	return sockets
}

func compactUserIDs(userIDs []int64) []int64 {
	out := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, uid := range userIDs {
		if uid <= 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

func (s *Server) wsRouteUserDirKey(userID int64) string {
	return fmt.Sprintf("ws:route:user:v3:%d", userID)
}

func (s *Server) wsRouteNodeKey(userID int64, nodeAddr string) string {
	return fmt.Sprintf("ws:route:user:v3:%d:%s", userID, nodeAddr)
}

func (s *Server) setUserRouteNode(userID int64, sessionVersion int64) {
	if userID <= 0 || sessionVersion <= 0 || s.deps.Redis == nil || s.wsNodeAdvertise == "" {
		return
	}
	ctx := context.Background()
	nodeKey := s.wsRouteNodeKey(userID, s.wsNodeAdvertise)
	dirKey := s.wsRouteUserDirKey(userID)
	sv := strconv.FormatInt(sessionVersion, 10)
	if err := s.deps.Redis.Set(ctx, nodeKey, sv, wsPresenceSessionTTL).Err(); err != nil {
		log.Printf("ws route set failed user_id=%d node=%s err=%v", userID, s.wsNodeAdvertise, err)
		return
	}
	if err := s.deps.Redis.SAdd(ctx, dirKey, s.wsNodeAdvertise).Err(); err != nil {
		log.Printf("ws route dir sadd failed user_id=%d node=%s err=%v", userID, s.wsNodeAdvertise, err)
	}
}

func (s *Server) refreshUserRouteNode(userID int64, sessionVersion int64) {
	if userID <= 0 || sessionVersion <= 0 || s.deps.Redis == nil || s.wsNodeAdvertise == "" {
		return
	}
	ctx := context.Background()
	nodeKey := s.wsRouteNodeKey(userID, s.wsNodeAdvertise)
	dirKey := s.wsRouteUserDirKey(userID)
	if err := s.deps.Redis.Expire(ctx, nodeKey, wsPresenceSessionTTL).Err(); err != nil {
		log.Printf("ws route refresh expire failed user_id=%d node=%s err=%v", userID, s.wsNodeAdvertise, err)
	}
	// Refresh the directory key TTL so it doesn't outlive its member node keys.
	// Each alive node refreshes this independently; when all nodes disconnect the
	// key expires naturally after wsPresenceSessionTTL.
	if err := s.deps.Redis.Expire(ctx, dirKey, wsPresenceSessionTTL).Err(); err != nil {
		log.Printf("ws route dir refresh expire failed user_id=%d err=%v", userID, err)
	}
}

// routeNodeDelScript atomically deletes the node route key only if its value
// matches the expected session version. This prevents a stale disconnect from
// removing a route key that a newer session (e.g. after security event) depends on.
// Returns: 1 = deleted, 0 = value mismatch (not deleted), -1 = key absent.
var routeNodeDelScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if current == false then
    return -1
end
if current ~= ARGV[1] then
    return 0
end
redis.call("DEL", KEYS[1])
return 1
`)

func (s *Server) clearUserRouteNodeIfCurrent(userID int64, sessionVersion int64) {
	if userID <= 0 || sessionVersion <= 0 || s.deps.Redis == nil || s.wsNodeAdvertise == "" {
		return
	}
	ctx := context.Background()
	nodeKey := s.wsRouteNodeKey(userID, s.wsNodeAdvertise)
	dirKey := s.wsRouteUserDirKey(userID)
	sv := strconv.FormatInt(sessionVersion, 10)
	result, err := routeNodeDelScript.Run(ctx, s.deps.Redis, []string{nodeKey}, sv).Int()
	if err != nil {
		log.Printf("ws route node key del script failed user_id=%d node=%s err=%v", userID, s.wsNodeAdvertise, err)
		return
	}
	if result != 1 {
		return
	}
	if err := s.deps.Redis.SRem(ctx, dirKey, s.wsNodeAdvertise).Err(); err != nil {
		log.Printf("ws route dir srem failed user_id=%d node=%s err=%v", userID, s.wsNodeAdvertise, err)
	}
	card, err := s.deps.Redis.SCard(ctx, dirKey).Result()
	if err == nil && card <= 0 {
		_ = s.deps.Redis.Del(ctx, dirKey).Err()
	}
}

func (s *Server) lookupUserRouteNodes(userID int64) []string {
	if userID <= 0 || s.deps.Redis == nil {
		return nil
	}
	ctx := context.Background()
	dirKey := s.wsRouteUserDirKey(userID)
	for i := 0; i < wsTargetLookupMaxRetries; i++ {
		members, err := s.deps.Redis.SMembers(ctx, dirKey).Result()
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				log.Printf("ws route lookup failed user_id=%d attempt=%d err=%v", userID, i+1, err)
			}
			time.Sleep(wsTargetLookupRetryDelay)
			continue
		}
		nodes := make([]string, 0, len(members))
		stale := make([]string, 0)
		for _, nodeAddr := range members {
			nodeAddr = strings.TrimSpace(nodeAddr)
			if nodeAddr == "" {
				continue
			}
			nodeKey := s.wsRouteNodeKey(userID, nodeAddr)
			exists, err := s.deps.Redis.Exists(ctx, nodeKey).Result()
			if err != nil {
				continue
			}
			if exists > 0 {
				nodes = append(nodes, nodeAddr)
			} else {
				stale = append(stale, nodeAddr)
			}
		}
		if len(stale) > 0 {
			pipe := s.deps.Redis.Pipeline()
			for _, nodeAddr := range stale {
				pipe.SRem(ctx, dirKey, nodeAddr)
			}
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("ws route stale-cleanup failed user_id=%d count=%d err=%v", userID, len(stale), err)
			}
		}
		return nodes
	}
	return nil
}

func (s *Server) wsNodeSubjectFor(nodeAddr string) string {
	sanitized := strings.NewReplacer(".", "_", ":", "_", "-", "_").Replace(strings.TrimSpace(nodeAddr))
	if sanitized == "" {
		sanitized = "unknown"
	}
	return s.wsSubject + ".node." + sanitized
}

func (s *Server) recordWSTargetDrop(userID int64, eventType, reason string) {
	log.Printf("ws target drop user_id=%d event=%s reason=%s", userID, eventType, reason)
	if s.deps.Redis == nil {
		return
	}
	key := "metrics:ws:target_drop:" + reason
	if _, err := s.deps.Redis.Incr(context.Background(), key).Result(); err != nil {
		log.Printf("ws target drop metric incr failed key=%s err=%v", key, err)
	}
}

func (s *Server) visibilityOfflineDelay() time.Duration {
	delayS := s.deps.Config.SharedGlobalSettings.Presence.VisibilityOfflineDelaySeconds
	if delayS <= 0 {
		return 0
	}
	return time.Duration(delayS) * time.Second
}

func (s *Server) offlineFastForwardWindow(presence store.PresenceState, now time.Time) (time.Time, int64) {
	if now.IsZero() {
		now = time.Now()
	}
	lastSeen := presence.LastSeenAt
	if lastSeen.IsZero() {
		return time.Time{}, 0
	}
	offlineStartedAt := lastSeen
	if normalizePresenceStatus(presence.Status) == "online" {
		offlineStartedAt = lastSeen.Add(s.visibilityOfflineDelay())
	}
	if !now.After(offlineStartedAt) {
		return time.Time{}, 0
	}
	elapsed := now.Sub(offlineStartedAt)
	if elapsed < time.Second {
		return time.Time{}, 0
	}
	return offlineStartedAt, int64(elapsed / time.Second)
}

func (s *Server) offlineStartedAtForStaleOnline(lastSeenAt time.Time, now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	if lastSeenAt.IsZero() {
		return now
	}
	offlineStartedAt := lastSeenAt.Add(s.visibilityOfflineDelay())
	if offlineStartedAt.After(now) {
		return now
	}
	return offlineStartedAt
}

func (s *Server) markOfflineFastForwardFromPresence(userID int64, presence store.PresenceState, now time.Time) (int64, error) {
	offlineStartedAt, offlineDurationS := s.offlineFastForwardWindow(presence, now)
	if offlineDurationS > 0 {
		if s.deps.Store != nil {
			state, ok := newOfflineFastForwardState(offlineStartedAt, offlineDurationS, now, s.offlineFastForwardMaxDuration())
			if ok {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				recovery, err := s.deps.Store.EnsureOfflineRecovery(ctx, userID, time.UnixMilli(state.OfflineStartedAtMS), time.UnixMilli(state.OfflineTargetAtMS), now)
				if err != nil {
					cancel()
					return 0, err
				}
				// Normal returns have only the last background interval to finish.
				// Finish that bounded tail before accepting new gameplay commands;
				// a server-outage backlog remains owned by the durable worker.
				if recovery.Status == "running" {
					if err := s.deps.Store.AdvanceOfflineRecovery(ctx, recovery, store.BuildActionSpecMap(s.deps.GameData)); err != nil {
						log.Printf("offline return tail deferred user_id=%d recovery_id=%d err=%v", userID, recovery.ID, err)
					}
				}
				cancel()
			}
		}
		s.markOfflineFastForward(userID, offlineStartedAt, offlineDurationS, now)
	}
	return offlineDurationS, nil
}

func (s *Server) presenceStatusTransitionWriteAt(current store.PresenceState, normalizedNext string, now time.Time) (time.Time, bool) {
	if now.IsZero() {
		now = time.Now()
	}
	if normalizedNext != "offline" || normalizePresenceStatus(current.Status) != "online" {
		return now, true
	}
	if !current.LastSeenAt.IsZero() && now.Before(current.LastSeenAt.Add(s.visibilityOfflineDelay())) {
		return time.Time{}, false
	}
	return s.offlineStartedAtForStaleOnline(current.LastSeenAt, now), true
}

func (s *Server) offlineFastForwardMaxDuration() time.Duration {
	maxSeconds := s.deps.Config.GlobalSettings.Activity.OfflineProgressMaxDurationSeconds
	if maxSeconds <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(maxSeconds) * time.Second
}

func newOfflineFastForwardState(startedAt time.Time, offlineDurationS int64, now time.Time, maxDuration time.Duration) (offlineFastForwardState, bool) {
	if offlineDurationS <= 0 {
		return offlineFastForwardState{}, false
	}
	if now.IsZero() {
		now = time.Now()
	}
	if startedAt.IsZero() {
		startedAt = now.Add(-time.Duration(offlineDurationS) * time.Second)
	}
	if !now.After(startedAt) {
		return offlineFastForwardState{}, false
	}
	elapsedS := int64(now.Sub(startedAt) / time.Second)
	if elapsedS <= 0 {
		elapsedS = offlineDurationS
	}
	if maxDuration <= 0 {
		maxDuration = 24 * time.Hour
	}
	maxS := int64(maxDuration / time.Second)
	if maxS <= 0 {
		maxS = int64((24 * time.Hour) / time.Second)
	}
	cappedS := elapsedS
	if cappedS > maxS {
		cappedS = maxS
	}
	if cappedS <= 0 {
		return offlineFastForwardState{}, false
	}
	targetAt := startedAt.Add(time.Duration(cappedS) * time.Second)
	if targetAt.After(now) {
		targetAt = now
		cappedS = int64(targetAt.Sub(startedAt) / time.Second)
	}
	if cappedS <= 0 {
		return offlineFastForwardState{}, false
	}
	return offlineFastForwardState{
		OfflineStartedAtMS:     startedAt.UnixMilli(),
		OfflineTargetAtMS:      targetAt.UnixMilli(),
		OfflineDurationSeconds: cappedS,
		CreatedAtMS:            now.UnixMilli(),
	}, true
}

func parseOfflineFastForwardState(raw string, now time.Time, maxDuration time.Duration) (offlineFastForwardState, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return offlineFastForwardState{}, false
	}
	if duration, err := strconv.ParseInt(raw, 10, 64); err == nil {
		state, ok := newOfflineFastForwardState(time.Time{}, duration, now, maxDuration)
		state.markerRaw = raw
		return state, ok
	}
	var state offlineFastForwardState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return offlineFastForwardState{}, false
	}
	if state.OfflineStartedAtMS <= 0 || state.OfflineTargetAtMS <= 0 || state.OfflineDurationSeconds <= 0 {
		return offlineFastForwardState{}, false
	}
	if state.OfflineTargetAtMS < state.OfflineStartedAtMS {
		return offlineFastForwardState{}, false
	}
	state.markerRaw = raw
	return state, true
}

func encodeOfflineFastForwardState(state offlineFastForwardState) string {
	raw, err := json.Marshal(state)
	if err != nil {
		return ""
	}
	return string(raw)
}

func offlineFastForwardStateExpired(state offlineFastForwardState, now time.Time) bool {
	if state.CreatedAtMS <= 0 {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(time.UnixMilli(state.CreatedAtMS).Add(offlineFastForwardReadyTTL))
}

func (s *Server) pruneExpiredOfflineFastForwardFallback(now time.Time) int {
	if s.deps.Redis != nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.offlineFFMu.Lock()
	defer s.offlineFFMu.Unlock()
	pruned := 0
	for userID, state := range s.offlineFFReady {
		if !offlineFastForwardStateExpired(state, now) {
			continue
		}
		delete(s.offlineFFReady, userID)
		pruned++
	}
	return pruned
}

func (s *Server) markOfflineFastForward(userID int64, offlineStartedAt time.Time, offlineDurationS int64, now time.Time) {
	if userID <= 0 || offlineDurationS <= 0 {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	nextState, ok := newOfflineFastForwardState(offlineStartedAt, offlineDurationS, now, s.offlineFastForwardMaxDuration())
	if !ok {
		return
	}
	if s.deps.Redis != nil {
		key := offlineFastForwardReadyKey(userID)
		currentRaw, err := s.deps.Redis.Get(context.Background(), key).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Printf("offline fastforward read failed user_id=%d err=%v", userID, err)
		}
		if currentState, ok := parseOfflineFastForwardState(currentRaw, now, s.offlineFastForwardMaxDuration()); ok && currentState.OfflineTargetAtMS > nextState.OfflineTargetAtMS {
			nextState = currentState
		}
		encoded := encodeOfflineFastForwardState(nextState)
		if encoded == "" {
			return
		}
		if err := s.deps.Redis.Set(context.Background(), key, encoded, offlineFastForwardReadyTTL).Err(); err != nil {
			log.Printf("offline fastforward mark failed user_id=%d err=%v", userID, err)
		}
		return
	}
	s.offlineFFMu.Lock()
	defer s.offlineFFMu.Unlock()
	if currentState, ready := s.offlineFFReady[userID]; ready && !offlineFastForwardStateExpired(currentState, now) && currentState.OfflineTargetAtMS > nextState.OfflineTargetAtMS {
		nextState = currentState
	}
	nextState.CreatedAtMS = now.UnixMilli()
	s.offlineFFReady[userID] = nextState
}

func (s *Server) hasOfflineFastForwardReady(userID int64) bool {
	if userID <= 0 {
		return false
	}
	if s.deps.Store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		pending, err := s.deps.Store.HasOfflineRecovery(ctx, userID)
		return err != nil || pending
	}
	if s.deps.Redis != nil {
		count, err := s.deps.Redis.Exists(context.Background(), offlineFastForwardReadyKey(userID)).Result()
		if err != nil {
			log.Printf("offline fastforward exists failed user_id=%d err=%v", userID, err)
			return false
		}
		return count > 0
	}
	s.offlineFFMu.Lock()
	defer s.offlineFFMu.Unlock()
	state, ready := s.offlineFFReady[userID]
	if ready && offlineFastForwardStateExpired(state, time.Now()) {
		delete(s.offlineFFReady, userID)
		return false
	}
	return ready
}

func (s *Server) peekOfflineFastForward(userID int64) (bool, offlineFastForwardState) {
	if userID <= 0 {
		return false, offlineFastForwardState{}
	}
	if s.deps.Redis != nil {
		raw, err := s.deps.Redis.Get(context.Background(), offlineFastForwardReadyKey(userID)).Result()
		if errors.Is(err, redis.Nil) {
			return false, offlineFastForwardState{}
		}
		if err != nil {
			log.Printf("offline fastforward peek failed user_id=%d err=%v", userID, err)
			return false, offlineFastForwardState{}
		}
		state, ok := parseOfflineFastForwardState(raw, time.Now(), s.offlineFastForwardMaxDuration())
		return ok, state
	}
	s.offlineFFMu.Lock()
	defer s.offlineFFMu.Unlock()
	state, ready := s.offlineFFReady[userID]
	if ready && offlineFastForwardStateExpired(state, time.Now()) {
		delete(s.offlineFFReady, userID)
		return false, offlineFastForwardState{}
	}
	return ready && state.OfflineDurationSeconds > 0 && state.OfflineTargetAtMS > 0, state
}

// consumeOfflineFastForwardIfUnchanged clears exactly the marker that produced
// the committed recovery result. A newer offline window must survive an older
// request finishing late.
func (s *Server) consumeOfflineFastForwardIfUnchanged(userID int64, expected offlineFastForwardState) (bool, offlineFastForwardState) {
	if userID <= 0 || expected.OfflineTargetAtMS <= 0 {
		return false, offlineFastForwardState{}
	}
	if s.deps.Redis != nil {
		key := offlineFastForwardReadyKey(userID)
		raw := strings.TrimSpace(expected.markerRaw)
		if raw == "" {
			raw = encodeOfflineFastForwardState(expected)
		}
		if raw == "" {
			return false, offlineFastForwardState{}
		}
		deleted, err := consumeOfflineFastForwardIfUnchangedScript.Run(context.Background(), s.deps.Redis, []string{key}, raw).Int()
		if err != nil {
			log.Printf("offline fastforward conditional consume failed user_id=%d err=%v", userID, err)
			return false, expected
		}
		return deleted == 1, expected
	}
	s.offlineFFMu.Lock()
	defer s.offlineFFMu.Unlock()
	state, ready := s.offlineFFReady[userID]
	if !ready || offlineFastForwardStateExpired(state, time.Now()) {
		if ready {
			delete(s.offlineFFReady, userID)
		}
		return false, offlineFastForwardState{}
	}
	if state.OfflineStartedAtMS != expected.OfflineStartedAtMS ||
		state.OfflineTargetAtMS != expected.OfflineTargetAtMS ||
		state.OfflineDurationSeconds != expected.OfflineDurationSeconds ||
		state.CreatedAtMS != expected.CreatedAtMS {
		return false, state
	}
	delete(s.offlineFFReady, userID)
	return true, state
}

func (s *Server) wsPresenceSessionConnKey(userID, sessionVersion int64) string {
	return actionruntime.PresenceSessionConnKey(userID, sessionVersion)
}

func (s *Server) markUserOnlineByWS(userID, sessionVersion int64) {
	if userID <= 0 || sessionVersion <= 0 {
		return
	}
	if s.deps.Redis != nil {
		key := s.wsPresenceSessionConnKey(userID, sessionVersion)
		if _, err := redisIncrExpireScript.Run(context.Background(), s.deps.Redis, []string{key}, int(wsPresenceSessionTTL/time.Second)).Result(); err != nil {
			log.Printf("ws presence incr failed user_id=%d err=%v", userID, err)
		}
	}
	if err := s.deps.Store.SetPresenceStatus(context.Background(), userID, "online"); err != nil {
		log.Printf("set presence online failed user_id=%d err=%v", userID, err)
		return
	}
	s.setUserPresenceCache(userID, "online")
	s.setUserRouteNode(userID, sessionVersion)
}

func (s *Server) refreshUserPresenceHeartbeat(userID, sessionVersion int64) {
	if userID <= 0 || sessionVersion <= 0 || s.deps.Redis == nil {
		return
	}
	key := s.wsPresenceSessionConnKey(userID, sessionVersion)
	if err := s.deps.Redis.Expire(context.Background(), key, wsPresenceSessionTTL).Err(); err != nil {
		log.Printf("ws presence heartbeat refresh failed user_id=%d err=%v", userID, err)
	}
	s.refreshUserRouteNode(userID, sessionVersion)
}

// redisIncrExpireScript atomically increments a key and sets its TTL in one call.
// This prevents a crash between INCR and EXPIRE from leaving a key with no TTL.
var redisIncrExpireScript = redis.NewScript(`
local result = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
return result
`)

// redisDecrFloorScript atomically decrements a counter only if > 0.
// Returns: -1 = key absent, 0 = already zero (no-op), 1 = decremented to zero (key deleted), 2 = decremented but still positive.
var redisDecrFloorScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if current == false then
    return -1
end
local n = tonumber(current)
if n == nil or n <= 0 then
    return 0
end
local result = redis.call("DECR", KEYS[1])
if result <= 0 then
    redis.call("DEL", KEYS[1])
    return 1
end
return 2
`)

func (s *Server) markUserOfflineByWS(userID, sessionVersion int64) {
	if userID <= 0 || sessionVersion <= 0 {
		return
	}
	if s.deps.Redis != nil {
		key := s.wsPresenceSessionConnKey(userID, sessionVersion)
		result, err := redisDecrFloorScript.Run(context.Background(), s.deps.Redis, []string{key}).Int()
		if err != nil {
			log.Printf("ws presence decr failed user_id=%d err=%v", userID, err)
		} else if result == 2 {
			return
		}
	}
	presence, err := s.deps.Store.GetPresence(context.Background(), userID)
	if err != nil {
		log.Printf("get presence for offline transition failed user_id=%d err=%v", userID, err)
		return
	}
	if presence.SessionVersion != sessionVersion {
		return
	}
	if len(s.userSessionSockets(userID, sessionVersion)) == 0 {
		if normalizePresenceStatus(presence.Status) == "offline" {
			s.setUserPresenceCache(userID, "offline")
			s.clearUserRouteNodeIfCurrent(userID, sessionVersion)
			return
		}
		if err := s.deps.Store.SetPresenceStatus(context.Background(), userID, "offline"); err != nil {
			log.Printf("set presence offline failed user_id=%d err=%v", userID, err)
			return
		}
		s.setUserPresenceCache(userID, "offline")
		s.clearUserRouteNodeIfCurrent(userID, sessionVersion)
	}
}

func (s *Server) hasActivePresenceSession(ctx context.Context, userID, sessionVersion int64) bool {
	if userID <= 0 || sessionVersion <= 0 {
		return false
	}
	if len(s.userSessionSockets(userID, sessionVersion)) > 0 {
		return true
	}
	if s.deps.Redis == nil {
		return false
	}
	count, err := s.deps.Redis.Get(ctx, s.wsPresenceSessionConnKey(userID, sessionVersion)).Int64()
	if errors.Is(err, redis.Nil) {
		return false
	}
	if err != nil {
		log.Printf("ws presence read failed user_id=%d err=%v", userID, err)
		return false
	}
	return count > 0
}

const (
	presenceReconcileInterval = 60 * time.Second
	presenceReconcileBatch    = 100
	presenceReconcileMaxBatch = 10
	presenceTouchDBInterval   = 30 * time.Second
)

type presenceReconcileResult struct {
	scanned      int
	corrected    int
	batches      int
	cursorUserID int64
	limited      bool
}

type listOnlinePresenceUsersFunc func(ctx context.Context, limit int, afterUserID int64) ([]store.OnlinePresenceRow, error)
type hasActivePresenceSessionFunc func(ctx context.Context, userID, sessionVersion int64) bool
type setPresenceOfflineFunc func(ctx context.Context, row store.OnlinePresenceRow) error
type setPresenceCacheFunc func(userID int64, status string)

// touchPresenceDB writes last_seen_at to DB at most once per presenceTouchDBInterval.
// presenceUpdatedAt is updated on every explicit status change (connect/disconnect/reconcile),
// so it doubles as the "last DB write" timestamp — no separate map needed.
// Hot path (>99% of calls): RLock only. Write lock only when the interval has expired.
func (s *Server) touchPresenceDB(userID int64) {
	s.presenceMu.RLock()
	skip := time.Since(s.presenceUpdatedAt[userID]) < presenceTouchDBInterval
	s.presenceMu.RUnlock()
	if skip {
		return
	}
	s.presenceMu.Lock()
	if time.Since(s.presenceUpdatedAt[userID]) < presenceTouchDBInterval {
		s.presenceMu.Unlock()
		return
	}
	s.presenceUpdatedAt[userID] = time.Now()
	s.presenceMu.Unlock()
	_ = s.deps.Store.TouchPresence(context.Background(), userID)
}

func (s *Server) startPresenceReconciler() {
	go s.presenceReconcileLoop()
}

func (s *Server) presenceReconcileLoop() {
	ticker := time.NewTicker(presenceReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.presenceReconcileStop:
			return
		case <-ticker.C:
			s.runPresenceReconcileBatch()
		}
	}
}

func (s *Server) runPresenceReconcileBatch() {
	ctx := context.Background()
	result, err := reconcileOnlinePresence(
		ctx,
		presenceReconcileBatch,
		presenceReconcileMaxBatch,
		s.presenceCursor,
		s.deps.Store.ListOnlinePresenceUsers,
		s.hasActivePresenceSession,
		func(ctx context.Context, row store.OnlinePresenceRow) error {
			offlineStartedAt := s.offlineStartedAtForStaleOnline(row.LastSeenAt, time.Now())
			return s.deps.Store.SetPresenceStatusAt(ctx, row.UserID, "offline", offlineStartedAt)
		},
		s.setUserPresenceCache,
	)
	if err != nil {
		log.Printf("presence reconcile query failed err=%v", err)
		return
	}
	if result.limited {
		s.presenceCursor = result.cursorUserID
	} else {
		s.presenceCursor = 0
	}
	if result.scanned > 0 || result.corrected > 0 {
		log.Printf(
			"presence reconcile scanned=%d corrected=%d batches=%d cursor_user_id=%d limited=%t",
			result.scanned,
			result.corrected,
			result.batches,
			result.cursorUserID,
			result.limited,
		)
	}
}

func reconcileOnlinePresence(
	ctx context.Context,
	batchSize int,
	maxBatches int,
	afterUserID int64,
	listOnline listOnlinePresenceUsersFunc,
	hasActive hasActivePresenceSessionFunc,
	setOffline setPresenceOfflineFunc,
	setCache setPresenceCacheFunc,
) (presenceReconcileResult, error) {
	if batchSize <= 0 {
		batchSize = presenceReconcileBatch
	}
	if maxBatches <= 0 {
		maxBatches = 1
	}
	result := presenceReconcileResult{}
	cursorUserID := afterUserID
	if cursorUserID < 0 {
		cursorUserID = 0
	}
	for result.batches < maxBatches {
		rows, err := listOnline(ctx, batchSize, cursorUserID)
		if err != nil {
			return result, err
		}
		if len(rows) == 0 {
			break
		}
		result.batches++
		for _, row := range rows {
			result.scanned++
			if row.UserID > cursorUserID {
				cursorUserID = row.UserID
				result.cursorUserID = cursorUserID
			}
			if hasActive(ctx, row.UserID, row.SessionVersion) {
				continue
			}
			if err := setOffline(ctx, row); err != nil {
				log.Printf("presence reconcile set offline failed user_id=%d err=%v", row.UserID, err)
				continue
			}
			setCache(row.UserID, "offline")
			result.corrected++
		}
		if len(rows) < batchSize {
			return result, nil
		}
	}
	result.limited = true
	return result, nil
}

func (s *Server) reconcilePresenceState(ctx context.Context, userID int64) (store.PresenceState, error) {
	state, err := s.deps.Store.GetPresence(ctx, userID)
	if err != nil {
		return store.PresenceState{}, err
	}
	expected := "offline"
	if s.hasActivePresenceSession(ctx, userID, state.SessionVersion) {
		expected = "online"
	}
	if state.Status != expected {
		statusAt := time.Now()
		if expected == "offline" && normalizePresenceStatus(state.Status) == "online" {
			statusAt = s.offlineStartedAtForStaleOnline(state.LastSeenAt, statusAt)
		}
		if err := s.deps.Store.SetPresenceStatusAt(ctx, userID, expected, statusAt); err != nil {
			return store.PresenceState{}, err
		}
		state.Status = expected
		state.LastSeenAt = statusAt
		state.UpdatedAt = time.Now()
	}
	s.setUserPresenceCache(userID, state.Status)
	return state, nil
}

func (s *Server) applyPresenceStatusTransition(ctx context.Context, userID int64, nextStatus string, now time.Time) (store.PresenceState, int64, error) {
	if userID <= 0 {
		return store.PresenceState{}, 0, errors.New("invalid user id")
	}
	normalizedNext := normalizePresenceStatus(nextStatus)
	current, err := s.deps.Store.GetPresence(ctx, userID)
	if err != nil {
		return store.PresenceState{}, 0, err
	}
	normalizedCurrent := normalizePresenceStatus(current.Status)
	offlineDurationS := int64(0)
	if normalizedCurrent == normalizedNext && normalizedNext != "online" {
		s.setUserPresenceCache(userID, normalizedCurrent)
		return current, 0, nil
	}
	if normalizedNext == "online" {
		offlineDurationS, err = s.markOfflineFastForwardFromPresence(userID, current, now)
		if err != nil {
			return current, 0, err
		}
	}
	statusAt, shouldWrite := s.presenceStatusTransitionWriteAt(current, normalizedNext, now)
	if !shouldWrite {
		s.setUserPresenceCache(userID, normalizedCurrent)
		return current, 0, nil
	}
	if err := s.deps.Store.SetPresenceStatusAt(ctx, userID, normalizedNext, statusAt); err != nil {
		return store.PresenceState{}, 0, err
	}
	current.Status = normalizedNext
	current.LastSeenAt = statusAt
	current.UpdatedAt = now
	s.setUserPresenceCache(userID, normalizedNext)
	return current, offlineDurationS, nil
}

func (s *Server) kickUserSockets(userID int64, reason string) {
	s.kickUserSocketsExcept(userID, reason, "")
}

func (s *Server) kickUserSocketsExcept(userID int64, reason string, excludeClientID string) {
	if userID <= 0 {
		return
	}
	s.publishWSBroadcastGlobalTargeted("auth.kicked", []int64{userID}, fiber.Map{"reason": reason}, excludeClientID)
}

func (s *Server) userHubClients(userID int64) []*ws.Client {
	if userID <= 0 {
		return nil
	}
	out := make([]*ws.Client, 0, 2)
	for _, client := range s.deps.Hub.Clients() {
		if client.UserID != userID {
			continue
		}
		out = append(out, client)
	}
	return out
}

func (s *Server) wsGlobalConnCountKey(userID int64) string {
	return fmt.Sprintf("ws:conn:global:%d", userID)
}

func (s *Server) incrGlobalConnectionCount(userID int64) {
	if userID <= 0 || s.deps.Redis == nil {
		return
	}
	key := s.wsGlobalConnCountKey(userID)
	if _, err := redisIncrExpireScript.Run(context.Background(), s.deps.Redis, []string{key}, int(wsPresenceSessionTTL/time.Second)).Result(); err != nil {
		log.Printf("ws global conn count incr failed user_id=%d err=%v", userID, err)
	}
}

func (s *Server) decrGlobalConnectionCount(userID int64) {
	if userID <= 0 || s.deps.Redis == nil {
		return
	}
	key := s.wsGlobalConnCountKey(userID)
	if _, err := redisDecrFloorScript.Run(context.Background(), s.deps.Redis, []string{key}).Int(); err != nil {
		log.Printf("ws global conn count decr failed user_id=%d err=%v", userID, err)
	}
}

// enforceConnectionLimit checks the global connection count across all nodes.
// If over the limit, evicts the oldest local connections (excluding excludeClientID)
// to bring the global count back to maxConns.
// In multi-node deployments, eviction targets local connections only — the evicted
// connection may be the newly-arrived one or an older local one.
func (s *Server) enforceConnectionLimit(userID int64, maxConns int, excludeClientID string) {
	if userID <= 0 || maxConns <= 0 || s.deps.Redis == nil {
		return
	}
	key := s.wsGlobalConnCountKey(userID)
	count, err := s.deps.Redis.Get(context.Background(), key).Int()
	if err != nil {
		return
	}
	over := count - maxConns
	if over <= 0 {
		return
	}
	clients := s.userHubClients(userID)
	if len(clients) == 0 {
		return
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].CreatedAt.Before(clients[j].CreatedAt)
	})
	kicked := 0
	for _, c := range clients {
		if kicked >= over {
			break
		}
		if c.ID == excludeClientID {
			continue
		}
		_ = s.writeWSEvent(c.Conn, "auth.kicked", fiber.Map{"reason": "CONNECTION_LIMIT"})
		_ = c.Conn.Close()
		kicked++
	}
	if kicked > 0 {
		log.Printf("ws connection limit enforced user_id=%d global_count=%d max=%d kicked_local=%d", userID, count, maxConns, kicked)
	}
}

func (s *Server) pushPlayerState(userID int64) {
	state, err := s.deps.Store.GetPlayerState(context.Background(), userID)
	if err != nil {
		return
	}
	s.pushUserEventClusterOnce(userID, "game.state.patch", state)
}

func (s *Server) pushWarehouseItems(userID int64) {
	s.pushInventoryInstances(userID)
}

func (s *Server) pushInventoryInstances(userID int64) {
	items, err := s.listInventoryInstances(context.Background(), userID, false)
	if err != nil {
		return
	}
	s.pushInventoryInstancesPatch(userID, items)
}

func (s *Server) listInventoryInstances(ctx context.Context, userID int64, allowCache bool) ([]store.ItemInstance, error) {
	cacheKey := fmt.Sprintf("warehouse:items:%d", userID)
	if allowCache && s.deps.Redis != nil {
		cached, cacheErr := s.deps.Redis.Get(ctx, cacheKey).Result()
		if cacheErr == nil && cached != "" {
			var items []store.ItemInstance
			if err := json.Unmarshal([]byte(cached), &items); err == nil {
				return items, nil
			}
		}
	}
	if !allowCache && s.deps.Redis != nil {
		_ = s.deps.Redis.Del(ctx, cacheKey).Err()
	}
	items, err := s.deps.Store.ListItemInstances(ctx, userID)
	if err != nil {
		return nil, err
	}
	if s.deps.Redis != nil {
		if payload, marshalErr := json.Marshal(items); marshalErr == nil {
			_ = s.deps.Redis.Set(ctx, cacheKey, string(payload), 30*time.Second).Err()
		}
	}
	return items, nil
}

func (s *Server) pushEquipmentDomain(userID int64) {
	s.pushEquipmentDomainWithMode(userID, false)
}

func (s *Server) pushEquipmentDomainSnapshot(userID int64) {
	s.pushEquipmentDomainWithMode(userID, true)
}

func (s *Server) pushEquipmentDomainWithMode(userID int64, fullSnapshot bool) {
	ctx := context.Background()
	defaultBasicSkillID := s.battleDefaultBasicSkillID()
	ownedSkills, ownedErr := s.deps.Store.ListOwnedSkills(ctx, userID)
	if ownedErr == nil {
		ownedSkillMap := make(map[string]string, len(ownedSkills)+1)
		for _, row := range ownedSkills {
			skillID := strings.TrimSpace(row.SkillID)
			if skillID == "" {
				continue
			}
			ownedSkillMap[strings.ToLower(skillID)] = skillID
		}
		if defaultBasicSkillID != "" {
			ownedSkillMap[strings.ToLower(defaultBasicSkillID)] = defaultBasicSkillID
		}
		if enableCtx, enableErr := s.loadBattleSkillEnableContext(ctx, userID); enableErr == nil {
			if config, cfgErr := s.loadCombatConfigOrDefault(ctx, userID); cfgErr == nil {
				currentSlots := battleAvailableSkillSlots(enableCtx.Attributes["intelligence"], s.battleSkillSlotRule())
				_, _, _ = s.reconcileCombatSkillConfigForRequirementsWithContext(
					ctx,
					userID,
					defaultBasicSkillID,
					ownedSkillMap,
					currentSlots,
					config,
					enableCtx,
				)
			}
		}
	}

	items, err := s.deps.Store.ListEquippedItems(context.Background(), userID)
	if err == nil {
		if fullSnapshot {
			s.pushEquipmentInstancesSnapshot(userID, items)
		} else {
			s.pushEquipmentInstancesPatch(userID, items)
		}
	}
	stats, err := s.deps.Store.GetPlayerStats(context.Background(), userID)
	if err == nil {
		seeds, seedErr := s.playerAttributeSeeds()
		if seedErr != nil {
			return
		}
		attributes := fiber.Map{}
		if ensureErr := s.deps.Store.EnsurePlayerAttributes(context.Background(), userID, seeds); ensureErr == nil {
			if states, listErr := s.deps.Store.ListPlayerAttributes(context.Background(), userID, seeds); listErr == nil {
				attributes = s.attributeMapFromStates(states)
			}
		}
		s.pushUserEventCluster(userID, "equipment.stats.updated", fiber.Map{
			"attributes": attributes,
			"derived":    stats.Derived,
			"updatedAt":  stats.UpdatedAt,
		})
	}
}
