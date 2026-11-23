package kittyworker

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/kittylogic"
	"mysecondwebgame/server/internal/store"
)

const (
	diaryContentIdleKey      = "game.catteryJournal.diary.idle"
	diaryContentWorkingKey   = "game.catteryJournal.diary.working"
	diaryAffectionGrantKey   = "game.catteryJournal.diary.affectionGrant"
	encounterContentMetKey   = "game.catteryJournal.encounter.met"
	journalTitleDiaryKey     = "game.catteryJournal.title.diary"
	journalTitleEncounterKey = "game.catteryJournal.title.encounter"
)

var (
	diaryIdleVariantKeys = []string{
		"game.catteryJournal.diary.idleVariants.1",
		"game.catteryJournal.diary.idleVariants.2",
		"game.catteryJournal.diary.idleVariants.3",
		"game.catteryJournal.diary.idleVariants.4",
		"game.catteryJournal.diary.idleVariants.5",
		"game.catteryJournal.diary.idleVariants.6",
		"game.catteryJournal.diary.idleVariants.7",
		"game.catteryJournal.diary.idleVariants.8",
		"game.catteryJournal.diary.idleVariants.9",
		"game.catteryJournal.diary.idleVariants.10",
		"game.catteryJournal.diary.idleVariants.11",
		"game.catteryJournal.diary.idleVariants.12",
		"game.catteryJournal.diary.idleVariants.13",
		"game.catteryJournal.diary.idleVariants.14",
		"game.catteryJournal.diary.idleVariants.15",
		"game.catteryJournal.diary.idleVariants.16",
		"game.catteryJournal.diary.idleVariants.17",
		"game.catteryJournal.diary.idleVariants.18",
		"game.catteryJournal.diary.idleVariants.19",
		"game.catteryJournal.diary.idleVariants.20",
	}
	diaryWorkingVariantKeys = []string{
		"game.catteryJournal.diary.workingVariants.1",
		"game.catteryJournal.diary.workingVariants.2",
		"game.catteryJournal.diary.workingVariants.3",
		"game.catteryJournal.diary.workingVariants.4",
		"game.catteryJournal.diary.workingVariants.5",
		"game.catteryJournal.diary.workingVariants.6",
		"game.catteryJournal.diary.workingVariants.7",
		"game.catteryJournal.diary.workingVariants.8",
		"game.catteryJournal.diary.workingVariants.9",
		"game.catteryJournal.diary.workingVariants.10",
		"game.catteryJournal.diary.workingVariants.11",
		"game.catteryJournal.diary.workingVariants.12",
		"game.catteryJournal.diary.workingVariants.13",
		"game.catteryJournal.diary.workingVariants.14",
		"game.catteryJournal.diary.workingVariants.15",
		"game.catteryJournal.diary.workingVariants.16",
		"game.catteryJournal.diary.workingVariants.17",
		"game.catteryJournal.diary.workingVariants.18",
		"game.catteryJournal.diary.workingVariants.19",
		"game.catteryJournal.diary.workingVariants.20",
	}
	encounterMetVariantKeys = []string{
		"game.catteryJournal.encounter.metVariants.1",
		"game.catteryJournal.encounter.metVariants.2",
		"game.catteryJournal.encounter.metVariants.3",
		"game.catteryJournal.encounter.metVariants.4",
		"game.catteryJournal.encounter.metVariants.5",
		"game.catteryJournal.encounter.metVariants.6",
		"game.catteryJournal.encounter.metVariants.7",
		"game.catteryJournal.encounter.metVariants.8",
		"game.catteryJournal.encounter.metVariants.9",
		"game.catteryJournal.encounter.metVariants.10",
		"game.catteryJournal.encounter.metVariants.11",
		"game.catteryJournal.encounter.metVariants.12",
		"game.catteryJournal.encounter.metVariants.13",
		"game.catteryJournal.encounter.metVariants.14",
		"game.catteryJournal.encounter.metVariants.15",
		"game.catteryJournal.encounter.metVariants.16",
		"game.catteryJournal.encounter.metVariants.17",
		"game.catteryJournal.encounter.metVariants.18",
		"game.catteryJournal.encounter.metVariants.19",
		"game.catteryJournal.encounter.metVariants.20",
	}
	cuteEquipmentVariantKeys = []string{
		"game.catteryJournal.diary.cuteEquipmentVariants.1",
		"game.catteryJournal.diary.cuteEquipmentVariants.2",
		"game.catteryJournal.diary.cuteEquipmentVariants.3",
		"game.catteryJournal.diary.cuteEquipmentVariants.4",
		"game.catteryJournal.diary.cuteEquipmentVariants.5",
		"game.catteryJournal.diary.cuteEquipmentVariants.6",
		"game.catteryJournal.diary.cuteEquipmentVariants.7",
		"game.catteryJournal.diary.cuteEquipmentVariants.8",
		"game.catteryJournal.diary.cuteEquipmentVariants.9",
		"game.catteryJournal.diary.cuteEquipmentVariants.10",
	}
)

type scheduledAffectionSlot struct {
	DateKey string
	SlotKey string
}

type Worker struct {
	store             *store.Store
	settings          config.KittyJournalSettings
	kitty             config.KittySettings
	rng               *rand.Rand
	personalityGrants []catalog.KittyJournalPersonalityGrant
}

func New(st *store.Store, settings config.KittyJournalSettings, kitty config.KittySettings, personalityGrants []catalog.KittyJournalPersonalityGrant) *Worker {
	return &Worker{
		store:             st,
		settings:          settings,
		kitty:             kitty,
		rng:               rand.New(rand.NewSource(time.Now().UnixNano())),
		personalityGrants: personalityGrants,
	}
}

func (w *Worker) resolvePersonalityGrant(contentKey, category string) *store.KittyJournalPersonalityGrantInput {
	contentKeyNorm := strings.TrimSpace(contentKey)
	categoryNorm := strings.TrimSpace(category)
	for _, g := range w.personalityGrants {
		if g.Chance <= 0 || len(g.PersonalityIDs) == 0 {
			continue
		}
		matchValue := strings.TrimSpace(g.MatchValue)
		switch strings.TrimSpace(g.MatchType) {
		case "contentKey":
			if matchValue == contentKeyNorm {
				return &store.KittyJournalPersonalityGrantInput{CandidateIDs: g.PersonalityIDs, Chance: g.Chance, RNG: w.rng}
			}
		case "category":
			if matchValue == categoryNorm {
				return &store.KittyJournalPersonalityGrantInput{CandidateIDs: g.PersonalityIDs, Chance: g.Chance, RNG: w.rng}
			}
		}
	}
	return nil
}

func (w *Worker) Run(ctx context.Context) {
	if !w.settings.Enabled {
		log.Printf("kitty worker disabled by config")
		<-ctx.Done()
		return
	}
	for {
		wait := w.nextDelay()
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := w.tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("kitty worker tick failed: %v", err)
		}
	}
}

func (w *Worker) nextDelay() time.Duration {
	base := time.Duration(w.settings.IntervalSeconds) * time.Second
	if base <= 0 {
		base = 30 * time.Minute
	}
	jitter := time.Duration(w.settings.JitterSeconds) * time.Second
	if jitter <= 0 {
		return base
	}
	offset := time.Duration(w.rng.Int63n(int64(jitter)*2+1)) - jitter
	if base+offset < time.Second {
		return time.Second
	}
	return base + offset
}

func (w *Worker) pickJournalKey(keys []string, fallback string) string {
	if len(keys) == 0 {
		return fallback
	}
	return keys[w.rng.Intn(len(keys))]
}

func (w *Worker) dueScheduledAffectionSlots(now time.Time) []scheduledAffectionSlot {
	settings := w.kitty.JournalRewards.ScheduledAffection
	if !settings.Enabled || len(settings.Times) == 0 {
		return nil
	}
	loc, err := time.LoadLocation(strings.TrimSpace(settings.Timezone))
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	localNow := now.In(loc)
	currentMinute := localNow.Hour()*60 + localNow.Minute()
	dateKey := localNow.Format("2006-01-02")
	out := make([]scheduledAffectionSlot, 0, len(settings.Times))
	for _, raw := range settings.Times {
		slotKey := strings.TrimSpace(raw)
		slotMinute, ok := parseHHMM(slotKey)
		if !ok || slotMinute > currentMinute {
			continue
		}
		out = append(out, scheduledAffectionSlot{DateKey: dateKey, SlotKey: slotKey})
	}
	return out
}

func parseHHMM(raw string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, ok := parseTwoDigitInt(parts[0])
	if !ok {
		return 0, false
	}
	minute, ok := parseTwoDigitInt(parts[1])
	if !ok || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func parseTwoDigitInt(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2 {
		return 0, false
	}
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		value = value*10 + int(ch-'0')
	}
	return value, true
}

func (w *Worker) rollCuteEquipmentDrop() (string, bool) {
	settings := w.kitty.JournalRewards.CuteEquipmentDrop
	if !settings.Enabled || settings.Chance <= 0 || len(settings.ItemIDs) == 0 {
		return "", false
	}
	if w.rng.Float64() > settings.Chance {
		return "", false
	}
	start := w.rng.Intn(len(settings.ItemIDs))
	for i := 0; i < len(settings.ItemIDs); i++ {
		itemID := strings.TrimSpace(settings.ItemIDs[(start+i)%len(settings.ItemIDs)])
		if itemID != "" {
			return itemID, true
		}
	}
	return "", false
}

func (w *Worker) rollRareItemDrops() ([]store.KittyRewardGrantSpec, string) {
	return w.rollRareItemDropSettings(w.kitty.JournalRewards.RareItemDrops)
}

func (w *Worker) rollEncounterRareItemDrops() ([]store.KittyRewardGrantSpec, string) {
	return w.rollRareItemDropSettings(w.kitty.JournalRewards.EncounterRareItemDrops)
}

func (w *Worker) rollRareItemDropSettings(drops []config.KittyRareItemDropSettings) ([]store.KittyRewardGrantSpec, string) {
	var rewards []store.KittyRewardGrantSpec
	contentKey := ""
	for _, drop := range drops {
		if !drop.Enabled || drop.Chance <= 0 || drop.Quantity <= 0 || strings.TrimSpace(drop.ItemID) == "" {
			continue
		}
		if w.rng.Float64() > drop.Chance {
			continue
		}
		rewards = append(rewards, store.KittyRewardGrantSpec{ItemID: strings.TrimSpace(drop.ItemID), Quantity: drop.Quantity})
		if contentKey == "" {
			contentKey = w.pickRareItemDropContentKey(drop)
		}
	}
	return rewards, contentKey
}

func (w *Worker) pickRareItemDropContentKey(drop config.KittyRareItemDropSettings) string {
	keys := make([]string, 0, len(drop.ContentKeys)+1)
	for _, key := range drop.ContentKeys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	if len(keys) > 0 {
		return w.pickJournalKey(keys, "")
	}
	return strings.TrimSpace(drop.ContentKey)
}

func (w *Worker) tick(ctx context.Context) error {
	candidates, err := w.store.ListAllKittyCandidates(ctx)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	var (
		scheduledWrites int
		diaryWrites     int
		encounterWrites int
	)
	scheduledSlots := w.dueScheduledAffectionSlots(time.Now())
	for _, candidate := range candidates {
		for _, slot := range scheduledSlots {
			settings := w.kitty.JournalRewards.ScheduledAffection
			rewards := []store.KittyRewardGrantSpec{{ItemID: strings.TrimSpace(settings.RewardItemID), Quantity: settings.Quantity}}
			contentKey := strings.TrimSpace(settings.ContentKey)
			if contentKey == "" {
				contentKey = diaryAffectionGrantKey
			}
			created, err := w.store.CreateKittyDiaryWithRewardsOnce(ctx, store.KittyJournalWriteInput{
				UserID:        candidate.UserID,
				KittyID:       candidate.Kitty.ID,
				EntryType:     store.KittyJournalEntryTypeDiary,
				TitleKey:      journalTitleDiaryKey,
				ContentKey:    contentKey,
				ContentParams: map[string]any{"kittyName": candidate.Kitty.Name},
				Reward:        kittylogic.RewardPayload(kittylogic.RewardResult{Items: rewards}),
			}, rewards, w.settings.MaxEntriesPerKitty, store.KittyJournalRewardClaimInput{
				RewardDate: slot.DateKey,
				RewardSlot: slot.SlotKey,
				RewardKey:  strings.TrimSpace(settings.RewardKey),
			})
			if err != nil {
				log.Printf("kitty scheduled affection grant failed user_id=%d kitty_id=%d slot=%s err=%v", candidate.UserID, candidate.Kitty.ID, slot.SlotKey, err)
				continue
			}
			if created {
				scheduledWrites++
			}
		}
		if w.rng.Float64() <= w.settings.EncounterProbability {
			target, targetErr := w.store.PickRandomEncounterTarget(ctx, candidate.UserID, candidate.Kitty.ID, w.rng)
			if targetErr == nil {
				baseEncounterContentKey := w.pickJournalKey(encounterMetVariantKeys, encounterContentMetKey)
				encounterRewards, rareContentKey := w.rollEncounterRareItemDrops()
				encounterContentKey := baseEncounterContentKey
				if rareContentKey != "" {
					encounterContentKey = rareContentKey
				}
				encounter := store.KittyEncounterWriteInput{
					FromUserID:  candidate.UserID,
					FromKittyID: candidate.Kitty.ID,
					ToUserID:    target.UserID,
					ToKittyID:   target.Kitty.ID,
					ContentKey:  encounterContentKey,
					ContentParams: map[string]any{
						"kittyName":       candidate.Kitty.Name,
						"targetKittyName": target.Kitty.Name,
					},
				}
				encounterGrant := w.resolvePersonalityGrant(baseEncounterContentKey, "encounter")
				fromJournal := store.KittyJournalWriteInput{
					UserID:     candidate.UserID,
					KittyID:    candidate.Kitty.ID,
					EntryType:  store.KittyJournalEntryTypeEncounter,
					TitleKey:   journalTitleEncounterKey,
					ContentKey: encounterContentKey,
					ContentParams: map[string]any{
						"kittyName":       candidate.Kitty.Name,
						"targetKittyName": target.Kitty.Name,
					},
					Reward:           kittylogic.RewardPayload(kittylogic.RewardResult{Items: encounterRewards}),
					PersonalityGrant: encounterGrant,
				}
				toJournal := store.KittyJournalWriteInput{
					UserID:     target.UserID,
					KittyID:    target.Kitty.ID,
					EntryType:  store.KittyJournalEntryTypeEncounter,
					TitleKey:   journalTitleEncounterKey,
					ContentKey: baseEncounterContentKey,
					ContentParams: map[string]any{
						"kittyName":       target.Kitty.Name,
						"targetKittyName": candidate.Kitty.Name,
					},
					PersonalityGrant: encounterGrant,
				}
				if err := w.store.CreateKittyEncounterWithDualJournalsAndRewards(ctx, encounter, fromJournal, toJournal, encounterRewards, w.settings.MaxEntriesPerKitty); err == nil {
					encounterWrites++
					continue
				}
			}
		}
		if w.rng.Float64() > w.settings.DiaryProbability {
			continue
		}
		actionID, err := w.store.GetKittyRunningActionID(ctx, candidate.UserID, candidate.Kitty.ID)
		if err != nil {
			continue
		}
		contentKey := w.pickJournalKey(diaryIdleVariantKeys, diaryContentIdleKey)
		contentParams := map[string]any{
			"kittyName": candidate.Kitty.Name,
		}
		if actionID != "" {
			contentKey = w.pickJournalKey(diaryWorkingVariantKeys, diaryContentWorkingKey)
			contentParams["actionId"] = actionID
		}
		mood := w.rng.Float64()*2 - 1
		category := "diary.idle"
		if actionID != "" {
			category = "diary.working"
		}
		var rewards []store.KittyRewardGrantSpec
		rewardPayload := map[string]any{}
		var kittyExp int64
		var bondExp int64
		var moodDelta float64
		rolledResult := kittylogic.RewardResult{}
		if poolID, pool, ok := kittylogic.ResolveJournalPool(w.kitty, contentKey, category); ok {
			rolled := kittylogic.RollRewardPool(poolID, pool, candidate.Kitty.Level, w.rng)
			rewards = rolled.Items
			kittyExp = rolled.KittyExp
			bondExp = rolled.BondExp
			moodDelta = rolled.MoodDelta
			rolledResult = rolled
		}
		if itemID, ok := w.rollCuteEquipmentDrop(); ok {
			rewards = append(rewards, store.KittyRewardGrantSpec{ItemID: itemID, Quantity: 1})
			contentKey = w.pickJournalKey(cuteEquipmentVariantKeys, diaryContentIdleKey)
		}
		if rareRewards, rareContentKey := w.rollRareItemDrops(); len(rareRewards) > 0 {
			rewards = append(rewards, rareRewards...)
			if rareContentKey != "" {
				contentKey = rareContentKey
			}
		}
		rolledResult.Items = rewards
		rolledResult.KittyExp = kittyExp
		rolledResult.BondExp = bondExp
		rolledResult.MoodDelta = moodDelta
		rewardPayload = kittylogic.RewardPayload(rolledResult)
		if err := w.store.CreateKittyDiaryWithRewards(ctx, store.KittyJournalWriteInput{
			UserID:           candidate.UserID,
			KittyID:          candidate.Kitty.ID,
			EntryType:        store.KittyJournalEntryTypeDiary,
			TitleKey:         journalTitleDiaryKey,
			ContentKey:       contentKey,
			ContentParams:    contentParams,
			Mood:             &mood,
			Reward:           rewardPayload,
			KittyExp:         kittyExp,
			BondExp:          bondExp,
			MoodDelta:        moodDelta,
			PersonalityGrant: w.resolvePersonalityGrant(contentKey, category),
		}, rewards, w.settings.MaxEntriesPerKitty); err == nil {
			diaryWrites++
		}
	}
	log.Printf("kitty worker tick done scanned=%d scheduledWrites=%d diaryWrites=%d encounterWrites=%d", len(candidates), scheduledWrites, diaryWrites, encounterWrites)
	return nil
}
