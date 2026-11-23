package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

const playerCombatSpecializationTreeID = "player_combat_specialization"

type specializationRuntimeVersion struct {
	TreeID          string `json:"treeId"`
	RulesetVersion  string `json:"rulesetVersion"`
	BalanceRevision int    `json:"balanceRevision"`
}

type battleWorkerSpecializationHeartbeat struct {
	WorkerID       string                        `json:"workerId"`
	RefreshedAt    string                        `json:"refreshedAt,omitempty"`
	Specialization *specializationRuntimeVersion `json:"specialization,omitempty"`
	MatchesLoaded  bool                          `json:"matchesLoaded"`
}

type battleWorkerSpecializationSnapshot struct {
	Available             bool                                  `json:"available"`
	Ready                 bool                                  `json:"ready"`
	WorkerCount           int                                   `json:"workerCount"`
	MatchingWorkers       int                                   `json:"matchingWorkers"`
	MismatchedWorkers     int                                   `json:"mismatchedWorkers"`
	MissingVersionWorkers int                                   `json:"missingVersionWorkers"`
	Workers               []battleWorkerSpecializationHeartbeat `json:"workers"`
}

var errBattleWorkerSpecializationUnavailable = errors.New("battle worker specialization heartbeat unavailable")

func (s *Server) specializationRuntimeVersion() (specializationRuntimeVersion, bool) {
	if s == nil {
		return specializationRuntimeVersion{}, false
	}
	tree := s.deps.GameData.SpecializationTreeByID(playerCombatSpecializationTreeID)
	if tree == nil || tree.Disabled {
		return specializationRuntimeVersion{}, false
	}
	version := specializationRuntimeVersion{
		TreeID:          strings.TrimSpace(tree.ID),
		RulesetVersion:  strings.TrimSpace(tree.RulesetVersion),
		BalanceRevision: tree.BalanceRevision,
	}
	if version.TreeID == "" || version.RulesetVersion == "" || version.BalanceRevision <= 0 {
		return specializationRuntimeVersion{}, false
	}
	return version, true
}

func (s *Server) specializationRuntimeVersionPayload() any {
	version, ok := s.specializationRuntimeVersion()
	if !ok {
		return nil
	}
	return version
}

func (s *Server) battleWorkerSpecializationSnapshot(ctx context.Context, expected specializationRuntimeVersion) (battleWorkerSpecializationSnapshot, error) {
	out := battleWorkerSpecializationSnapshot{Workers: []battleWorkerSpecializationHeartbeat{}}
	if s == nil || s.deps.Redis == nil {
		return out, errBattleWorkerSpecializationUnavailable
	}

	pattern := s.battleWorkerHeartbeatPattern()
	keyPrefix := strings.TrimSuffix(pattern, "*")
	keySet := map[string]struct{}{}
	var cursor uint64
	for {
		keys, nextCursor, err := s.deps.Redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return out, errBattleWorkerSpecializationUnavailable
		}
		for _, key := range keys {
			keySet[key] = struct{}{}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		out.Available = true
		return out, nil
	}

	values, err := s.deps.Redis.MGet(ctx, keys...).Result()
	if err != nil {
		return out, errBattleWorkerSpecializationUnavailable
	}
	for index, raw := range values {
		fallbackWorkerID := strings.TrimSpace(strings.TrimPrefix(keys[index], keyPrefix))
		heartbeat := battleWorkerSpecializationHeartbeat{WorkerID: fallbackWorkerID}
		if encoded, ok := raw.(string); ok {
			var stored struct {
				WorkerID       string                        `json:"workerId"`
				RefreshedAt    string                        `json:"refreshedAt"`
				Specialization *specializationRuntimeVersion `json:"specialization"`
			}
			if json.Unmarshal([]byte(encoded), &stored) == nil {
				if value := strings.TrimSpace(stored.WorkerID); value != "" {
					heartbeat.WorkerID = value
				}
				heartbeat.RefreshedAt = strings.TrimSpace(stored.RefreshedAt)
				heartbeat.Specialization = stored.Specialization
			}
		}
		if heartbeat.Specialization == nil || heartbeat.Specialization.TreeID == "" || heartbeat.Specialization.RulesetVersion == "" || heartbeat.Specialization.BalanceRevision <= 0 {
			out.MissingVersionWorkers++
		} else if *heartbeat.Specialization == expected {
			heartbeat.MatchesLoaded = true
			out.MatchingWorkers++
		} else {
			out.MismatchedWorkers++
		}
		out.Workers = append(out.Workers, heartbeat)
	}
	sort.Slice(out.Workers, func(i, j int) bool { return out.Workers[i].WorkerID < out.Workers[j].WorkerID })
	out.Available = true
	out.WorkerCount = len(out.Workers)
	out.Ready = out.WorkerCount > 0 && out.MatchingWorkers == out.WorkerCount
	return out, nil
}

func (s *Server) battleWorkerSpecializationSnapshotPayload(ctx context.Context) battleWorkerSpecializationSnapshot {
	expected, ok := s.specializationRuntimeVersion()
	if !ok {
		return battleWorkerSpecializationSnapshot{Workers: []battleWorkerSpecializationHeartbeat{}}
	}
	snapshot, err := s.battleWorkerSpecializationSnapshot(ctx, expected)
	if err != nil {
		return battleWorkerSpecializationSnapshot{Workers: []battleWorkerSpecializationHeartbeat{}}
	}
	return snapshot
}
