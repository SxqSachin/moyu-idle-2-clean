package httpapi

import "github.com/gofiber/fiber/v2"

func (s *Server) handleBattleMetricsGet(c *fiber.Ctx) error {
	snapshot := s.battleMetrics.Snapshot()
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{
		"metrics": snapshot,
		"serverMemory": fiber.Map{
			"mapSizes": s.memoryMapSizeSnapshot(),
		},
		"runtime": fiber.Map{
			"appEnv":                                 s.deps.Config.AppEnv,
			"globalConfigPath":                       s.deps.Config.GlobalConfigPath,
			"wsBattleBatchCompressionPublishEnabled": s.deps.Config.WSBattleBatchCompression,
			"specialization":                         s.specializationRuntimeVersionPayload(),
			"battleWorkers":                          s.battleWorkerSpecializationSnapshotPayload(c.Context()),
			"battle": fiber.Map{
				"timelinePushIntervalMs":             s.deps.Config.GlobalSettings.Battle.TimelinePushIntervalMs,
				"playbackFixedIntervalMs":            s.deps.Config.GlobalSettings.Battle.PlaybackFixedIntervalMs,
				"playbackIntervalJitterTolerancePct": s.deps.Config.GlobalSettings.Battle.PlaybackIntervalJitterTolerancePct,
				"settlementRequirePlaybackFinished":  s.deps.Config.GlobalSettings.Battle.SettlementRequirePlaybackFinished,
				"interBattleIntervalMs":              s.deps.Config.GlobalSettings.Battle.InterBattleIntervalMs,
				"compileMaxExecMs":                   s.deps.Config.GlobalSettings.Battle.CompileMaxExecMs,
				"playbackMaxEventsBatch":             s.deps.Config.GlobalSettings.Battle.PlaybackMaxEventsBatch,
				"tickShardWorkers":                   s.deps.Config.GlobalSettings.Battle.TickShardWorkers,
				"tickClaimRoomsMin":                  s.deps.Config.GlobalSettings.Battle.TickClaimRoomsMin,
				"tickClaimRoomsMax":                  s.deps.Config.GlobalSettings.Battle.TickClaimRoomsMax,
				"tickTargetLeaseUtilPct":             s.deps.Config.GlobalSettings.Battle.TickTargetLeaseUtilPct,
				"settlementWorkers":                  s.deps.Config.GlobalSettings.Battle.SettlementWorkers,
				"skillExpWorkers":                    s.deps.Config.GlobalSettings.Battle.SkillExpWorkers,
				"attrExpWorkers":                     s.deps.Config.GlobalSettings.Battle.AttrExpWorkers,
				"finalizeSlots":                      s.deps.Config.GlobalSettings.Battle.FinalizeSlots,
			},
		},
	})
}
