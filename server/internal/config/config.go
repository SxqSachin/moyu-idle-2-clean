package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv                       string
	ReleaseAnnounceToken         string
	ReleaseBuildID               string
	ReleaseReloadDelaySec        int
	WSMessageKey                 string
	WSTargetedRouting            bool
	WSBattleBatchCompression     bool
	Port                         string
	DatabaseURL                  string
	AuthDBPoolMaxConns           int32
	AuthDBTimeoutMS              int
	RedisAddr                    string
	RedisPassword                string
	RedisDB                      int
	NATSURL                      string
	MQTopicPrefix                string
	SchedulerShards              int
	SchedulerEngine              string
	SchedulerTickMS              int
	SchedulerWheelSize           int
	OrdinarySettlementEnabled    bool
	AchievementsEnabled          bool
	AchievementWorkerEnabled     bool
	AchievementWorkerConcurrency int
	AchievementBatchSize         int
	AchievementPollIntervalMS    int
	AchievementCleanupIntervalMS int
	NodeAdvertiseAddr            string
	NodeRegistryPrefix           string
	NodeHeartbeatSec             int
	NodeHeartbeatIntSec          int
	GameDataPath                 string
	GlobalConfigPath             string
	SharedGlobalConfigPath       string
	KittyConfigPath              string
	CatVillageConfigPath         string
	GlobalSettings               GlobalSettings
	SharedGlobalSettings         SharedGlobalSettings
	KittySettings                KittySettings
	CatVillageSettings           CatVillageSettings
	JWTSecret                    string
	RiskPasswordPepper           string
	GeetestID                    string
	GeetestKey                   string
	GeetestBypassURL             string
	GeetestBypassCycleSec        int
	GeetestBypassStatusKey       string
	GeetestDevBypass             bool
	GeetestTokenTTLSec           int
	RealIPProxyHeader            string
	RealIPTrustedCheck           bool
	RealIPTrustedProxies         []string
	RealIPEnableValidation       bool
	MaxConnectionsPerUser        int
}

func Load() (Config, error) {
	appEnv := strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "development")))
	globalConfigPath := getenv("GLOBAL_CONFIG_PATH", "../packages/game-config/source/global-config.json")
	globalSettings, err := LoadGlobalSettings(globalConfigPath)
	if err != nil {
		return Config{}, err
	}
	sharedGlobalConfigPath := getenv("SHARED_GLOBAL_CONFIG_PATH", "../packages/game-config/source/shared-global-config.json")
	sharedGlobalSettings, err := LoadSharedGlobalSettings(sharedGlobalConfigPath)
	if err != nil {
		return Config{}, err
	}
	kittyConfigPath := getenv("KITTY_CONFIG_PATH", "../packages/game-config/source/kitty-config.json")
	kittySettings, err := LoadKittySettings(kittyConfigPath)
	if err != nil {
		return Config{}, err
	}
	catVillageConfigPath := getenv("CAT_VILLAGE_CONFIG_PATH", "../packages/game-config/source/cat-village-config.json")
	catVillageSettings, err := LoadCatVillageSettings(catVillageConfigPath)
	if err != nil {
		return Config{}, err
	}
	releaseBuildID := strings.TrimSpace(getenv("RELEASE_BUILD_ID", globalSettings.Release.BuildID))
	releaseReloadDelaySec := atoi(getenv("RELEASE_RELOAD_DELAY_SECONDS", strconv.Itoa(globalSettings.Release.DefaultReloadDelaySeconds)), globalSettings.Release.DefaultReloadDelaySeconds)
	globalSettings.ContentModeration.VocabularyJSONPath = resolvePathOverride(
		getenv("CONTENT_MODERATION_VOCABULARY_JSON_PATH", globalSettings.ContentModeration.VocabularyJSONPath),
		globalConfigPath,
	)
	globalSettings.EmailDomainPolicy.BlocklistPath = resolvePathOverride(
		getenv("EMAIL_DOMAIN_BLOCKLIST_PATH", globalSettings.EmailDomainPolicy.BlocklistPath),
		globalConfigPath,
	)
	return Config{
		AppEnv:                       appEnv,
		ReleaseAnnounceToken:         getenv("RELEASE_ANNOUNCE_TOKEN", ""),
		ReleaseBuildID:               releaseBuildID,
		ReleaseReloadDelaySec:        releaseReloadDelaySec,
		WSMessageKey:                 getenv("WS_MESSAGE_KEY", "mysecondwebgame-dev-ws-key"),
		WSTargetedRouting:            getenv("WS_TARGETED_ROUTING_ENABLED", "false") == "true",
		WSBattleBatchCompression:     getenv("WS_BATTLE_BATCH_COMPRESSION_ENABLED", "false") == "true",
		Port:                         getenv("APP_PORT", "8080"),
		DatabaseURL:                  getenv("DATABASE_URL", "postgres://game:game@localhost:5432/game?sslmode=disable"),
		AuthDBPoolMaxConns:           int32(clampMin(atoi(getenv("AUTH_DB_POOL_MAX_CONNS", "4"), 4), 1)),
		AuthDBTimeoutMS:              clampMin(atoi(getenv("AUTH_DB_TIMEOUT_MS", "800"), 800), 100),
		RedisAddr:                    getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:                getenv("REDIS_PASSWORD", ""),
		RedisDB:                      atoi(getenv("REDIS_DB", "0"), 0),
		NATSURL:                      getenv("NATS_URL", "nats://localhost:4222"),
		MQTopicPrefix:                getenv("MQ_TOPIC_PREFIX", "game"),
		SchedulerShards:              atoi(getenv("SCHEDULER_SHARDS", "8"), 8),
		SchedulerEngine:              getenv("SCHEDULER_ENGINE", "minheap"),
		SchedulerTickMS:              atoi(getenv("SCHEDULER_TICK_MS", "100"), 100),
		SchedulerWheelSize:           atoi(getenv("SCHEDULER_WHEEL_SIZE", "256"), 256),
		OrdinarySettlementEnabled:    getenv("ORDINARY_SETTLEMENT_ENABLED", "false") == "true",
		AchievementsEnabled:          getenv("ACHIEVEMENTS_ENABLED", "true") != "false",
		AchievementWorkerEnabled:     getenv("ACHIEVEMENT_WORKER_ENABLED", "true") != "false",
		AchievementWorkerConcurrency: clampMin(atoi(getenv("ACHIEVEMENT_WORKER_CONCURRENCY", "1"), 1), 1),
		AchievementBatchSize:         clampMin(atoi(getenv("ACHIEVEMENT_BATCH_SIZE", "500"), 500), 1),
		AchievementPollIntervalMS:    clampMin(atoi(getenv("ACHIEVEMENT_POLL_INTERVAL_MS", "500"), 500), 100),
		AchievementCleanupIntervalMS: clampMin(atoi(getenv("ACHIEVEMENT_CLEANUP_INTERVAL_MS", "300000"), 300000), 60000),
		NodeAdvertiseAddr:            getenv("NODE_ADVERTISE_ADDR", ""),
		NodeRegistryPrefix:           getenv("NODE_REGISTRY_PREFIX", "ws:backend:node:"),
		NodeHeartbeatSec:             atoi(getenv("NODE_HEARTBEAT_SECONDS", "30"), 30),
		NodeHeartbeatIntSec:          atoi(getenv("NODE_HEARTBEAT_INTERVAL_SECONDS", "10"), 10),
		GameDataPath:                 getenv("GAME_DATA_PATH", "../packages/game-data/generated/game-data.json"),
		GlobalConfigPath:             globalConfigPath,
		SharedGlobalConfigPath:       sharedGlobalConfigPath,
		KittyConfigPath:              kittyConfigPath,
		CatVillageConfigPath:         catVillageConfigPath,
		GlobalSettings:               globalSettings,
		SharedGlobalSettings:         sharedGlobalSettings,
		KittySettings:                kittySettings,
		CatVillageSettings:           catVillageSettings,
		JWTSecret:                    getenv("JWT_SECRET", "dev-secret"),
		RiskPasswordPepper:           getenv("RISK_PASSWORD_PEPPER", getenv("JWT_SECRET", "dev-secret")),
		GeetestID:                    getenv("GEETEST_ID", ""),
		GeetestKey:                   getenv("GEETEST_KEY", ""),
		GeetestBypassURL:             getenv("GEETEST_BYPASS_URL", "http://bypass.geetest.com/v1/bypass_status.php"),
		GeetestBypassCycleSec:        atoi(getenv("GEETEST_BYPASS_CYCLE_SECONDS", "10"), 10),
		GeetestBypassStatusKey:       getenv("GEETEST_BYPASS_STATUS_KEY", "gt_server_bypass_status"),
		GeetestDevBypass:             getenv("GEETEST_DEV_BYPASS", "true") == "true",
		GeetestTokenTTLSec:           atoi(getenv("GEETEST_TOKEN_TTL_SECONDS", "120"), 120),
		RealIPProxyHeader:            getenv("REAL_IP_PROXY_HEADER", ""),
		RealIPTrustedCheck:           getenv("REAL_IP_TRUSTED_PROXY_CHECK", "false") == "true",
		RealIPTrustedProxies:         parseCSV(getenv("REAL_IP_TRUSTED_PROXIES", "")),
		RealIPEnableValidation:       getenv("REAL_IP_ENABLE_VALIDATION", "true") != "false",
		MaxConnectionsPerUser:        clampMin(atoi(getenv("MAX_CONNECTIONS_PER_USER", "3"), 3), 1),
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoi(v string, fallback int) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func clampMin(v, min int) int {
	if v < min {
		return min
	}
	return v
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func resolvePathOverride(raw string, configPath string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(configPath), raw))
}
