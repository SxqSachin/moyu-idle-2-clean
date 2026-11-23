package httpapi

import (
	"sort"
	"time"
)

// mutationSyncScope defines which user domains should be pushed after a mutation.
type mutationSyncScope struct {
	UserIDs          []int64
	PlayerState      bool
	Inventory        bool
	Equipment        bool
	Attributes       bool
	Queue            bool
	KittyIDs         []int64
	KittyEquipment   bool
	KittyAttributes  bool
	Wardrobe         bool
	Achievements     bool
	OperationEffects bool
}

type mutationSyncFlags struct {
	playerState      bool
	inventory        bool
	equipment        bool
	attributes       bool
	queue            bool
	kittyEquipment   bool
	kittyAttributes  bool
	kittyIDs         map[int64]struct{}
	wardrobe         bool
	achievements     bool
	operationEffects bool
}

const mutationSyncFlushInterval = 25 * time.Millisecond

func (s *Server) syncAfterMutation(scope mutationSyncScope) {
	if len(scope.UserIDs) == 0 {
		return
	}
	if s.mutationSyncQueue == nil {
		s.syncAfterMutationNow(scope)
		return
	}
	select {
	case s.mutationSyncQueue <- scope:
	default:
		// Keep mutation request path non-blocking even under burst traffic.
		if s.battleLoopEnabled {
			s.launchBattleTaskOrRun(func() { s.syncAfterMutationNow(scope) })
		} else {
			go s.syncAfterMutationNow(scope)
		}
	}
}

func (s *Server) startMutationSyncWorker() {
	if s.mutationSyncQueue == nil {
		return
	}
	if s.battleLoopEnabled {
		s.launchBattleTask(s.mutationSyncWorkerLoop)
		return
	}
	go s.mutationSyncWorkerLoop()
}

func (s *Server) mutationSyncWorkerLoop() {
	ticker := time.NewTicker(mutationSyncFlushInterval)
	defer ticker.Stop()
	pending := make(map[int64]mutationSyncFlags, 64)
	flush := func() {
		for userID, flags := range pending {
			s.syncAfterMutationNow(mutationSyncScope{
				UserIDs:          []int64{userID},
				PlayerState:      flags.playerState,
				Inventory:        flags.inventory,
				Equipment:        flags.equipment,
				Attributes:       flags.attributes,
				Queue:            flags.queue,
				KittyIDs:         keysFromInt64Set(flags.kittyIDs),
				KittyEquipment:   flags.kittyEquipment,
				KittyAttributes:  flags.kittyAttributes,
				Wardrobe:         flags.wardrobe,
				Achievements:     flags.achievements,
				OperationEffects: flags.operationEffects,
			})
			delete(pending, userID)
		}
	}
	for {
		select {
		case scope := <-s.mutationSyncQueue:
			for _, userID := range scope.UserIDs {
				if userID <= 0 {
					continue
				}
				flags := pending[userID]
				flags.playerState = flags.playerState || scope.PlayerState
				flags.inventory = flags.inventory || scope.Inventory
				flags.equipment = flags.equipment || scope.Equipment
				flags.attributes = flags.attributes || scope.Attributes
				flags.queue = flags.queue || scope.Queue
				flags.kittyEquipment = flags.kittyEquipment || scope.KittyEquipment
				flags.kittyAttributes = flags.kittyAttributes || scope.KittyAttributes
				flags.wardrobe = flags.wardrobe || scope.Wardrobe
				flags.achievements = flags.achievements || scope.Achievements
				flags.operationEffects = flags.operationEffects || scope.OperationEffects
				if len(scope.KittyIDs) > 0 {
					if flags.kittyIDs == nil {
						flags.kittyIDs = make(map[int64]struct{}, len(scope.KittyIDs))
					}
					for _, kittyID := range scope.KittyIDs {
						if kittyID > 0 {
							flags.kittyIDs[kittyID] = struct{}{}
						}
					}
				}
				pending[userID] = flags
			}
		case <-ticker.C:
			flush()
		case <-s.battleStopCh:
			flush()
			return
		}
	}
}

func (s *Server) syncAfterMutationNow(scope mutationSyncScope) {
	if len(scope.UserIDs) == 0 {
		return
	}
	unique := make(map[int64]struct{}, len(scope.UserIDs))
	for _, userID := range scope.UserIDs {
		if userID <= 0 {
			continue
		}
		if _, exists := unique[userID]; exists {
			continue
		}
		unique[userID] = struct{}{}
	}
	for userID := range unique {
		if scope.PlayerState {
			s.pushPlayerState(userID)
		}
		if scope.Inventory {
			s.pushWarehouseItems(userID)
		}
		if scope.Equipment {
			s.pushEquipmentDomain(userID)
		}
		if scope.Attributes {
			s.pushPlayerAttributes(userID)
		}
		if scope.Queue || scope.Attributes || scope.Equipment {
			s.pushActionQueueSnapshot(userID)
		}
		if scope.KittyAttributes {
			for _, kittyID := range compactInt64s(scope.KittyIDs) {
				s.pushKittyAttributes(userID, kittyID)
			}
		}
		if scope.KittyEquipment {
			for _, kittyID := range compactInt64s(scope.KittyIDs) {
				s.pushKittyEquipment(userID, kittyID)
			}
		}
		if (scope.KittyAttributes || scope.KittyEquipment) && len(scope.KittyIDs) > 0 {
			s.pushActionQueueSnapshot(userID)
		}
		if scope.Wardrobe {
			s.pushWardrobe(userID)
		}
		if scope.Achievements {
			s.pushAchievements(userID)
		}
		if scope.OperationEffects {
			s.pushOperationEffectsSnapshot(userID)
		}
	}
}

func compactInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func keysFromInt64Set(set map[int64]struct{}) []int64 {
	if len(set) == 0 {
		return nil
	}
	out := make([]int64, 0, len(set))
	for value := range set {
		if value > 0 {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
