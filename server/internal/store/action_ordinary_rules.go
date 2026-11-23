package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	randv2 "math/rand/v2"
	"sort"
	"strconv"
	"strings"

	"mysecondwebgame/server/internal/actionsettlement"
	"mysecondwebgame/server/internal/catalog"
)

const ordinarySettlementVersion = "ordinary-statistical-v1"

type ordinarySettlementState struct {
	Version       string                       `json:"version"`
	RandomVersion string                       `json:"randomVersion"`
	Seed          string                       `json:"seed"`
	FirstOrdinal  int                          `json:"firstOrdinal"`
	Spec          ActionSpec                   `json:"spec"`
	Grants        map[string]ordinaryGrantRule `json:"grants"`
	FishRecipe    *catalog.FishFoodRecipe      `json:"fishRecipe,omitempty"`
}

type ordinaryGrantRule struct {
	Stackable      bool                               `json:"stackable"`
	Commodity      bool                               `json:"commodity"`
	Durability     int64                              `json:"durability"`
	DefaultAffixes []EquipmentAffixSlot               `json:"defaultAffixes"`
	Rolls          []ordinaryAffixRoll                `json:"rolls,omitempty"`
	EnhanceLevel   int                                `json:"enhanceLevel,omitempty"`
	EnhancedBase   map[string]float64                 `json:"enhancedBase,omitempty"`
	Definitions    map[string]catalog.AffixDefinition `json:"definitions,omitempty"`
}

type ordinaryAffixRoll struct {
	Slot       int                     `json:"slot"`
	Definition catalog.AffixDefinition `json:"definition"`
	Rarities   []ordinaryRarityWeight  `json:"rarities"`
}

type ordinaryRarityWeight struct {
	ID     string  `json:"id"`
	Weight float64 `json:"weight"`
}

func ordinaryMainType(mainType string) bool {
	switch mainType {
	case "collecting", "mining", "farming_animal", "crafting", "forging", "sewing", "cooking":
		return true
	default:
		return false
	}
}

func ordinaryState(raw []byte) (*ordinarySettlementState, error) {
	var envelope queueRuntimeEnvelope
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	state := envelope.OrdinarySettlement
	if state == nil {
		return nil, nil
	}
	if state.Version != ordinarySettlementVersion || state.RandomVersion != actionsettlement.RandomVersion {
		return nil, errors.New("unsupported ordinary settlement version")
	}
	seed, err := hex.DecodeString(state.Seed)
	if err != nil || len(seed) != 32 || state.FirstOrdinal < 0 || state.Spec.ID != envelope.ActionID || !ordinaryMainType(state.Spec.MainType) {
		return nil, errors.New("invalid ordinary settlement checkpoint")
	}
	return state, nil
}

func (state *ordinarySettlementState) random() *actionsettlement.Random {
	raw, _ := hex.DecodeString(state.Seed)
	var seed [32]byte
	copy(seed[:], raw)
	return actionsettlement.NewRandom(seed)
}

func (s *Store) freezeOrdinarySettlement(item QueueRuntimeItem, raw []byte) (json.RawMessage, error) {
	state, err := ordinaryState(raw)
	if err != nil || state != nil || !s.config.OrdinarySettlementEnabled || !ordinaryMainType(item.Spec.MainType) {
		return raw, err
	}
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	state = &ordinarySettlementState{
		Version: ordinarySettlementVersion, RandomVersion: actionsettlement.RandomVersion,
		Seed: hex.EncodeToString(seed[:]), FirstOrdinal: item.DoneCount, Spec: item.Spec,
		Grants: make(map[string]ordinaryGrantRule),
	}
	state.FishRecipe = s.gameData.FishFoodRecipeByActionID(item.ActionID)
	if state.FishRecipe != nil {
		one := int64(1)
		state.Spec.ItemReward = map[string]ActionItemRewardSpec{state.FishRecipe.OutputItemID: {Count: &one}}
		item.Spec = state.Spec
	}
	ids := make([]string, 0, len(item.Spec.ItemReward)+len(item.Spec.Costs)+1)
	for id, reward := range item.Spec.ItemReward {
		if err := validateOrdinaryReward(reward); err != nil {
			return nil, fmt.Errorf("ordinary reward %s: %w", id, err)
		}
		ids = append(ids, id)
	}
	for id, quantity := range item.Spec.Costs {
		if quantity < 0 {
			return nil, ErrBadQuantity
		}
		ids = append(ids, id)
	}
	if item.Spec.MainOutput != "" {
		ids = append(ids, item.Spec.MainOutput)
	}
	for _, id := range normalizeTemplateItemIDs(ids) {
		rule, err := s.freezeOrdinaryGrant(id, item.Spec.ItemReward[id])
		if err != nil {
			return nil, err
		}
		run, _ := parseGenericRunContext(raw)
		if run.UseInstanceBindings && run.InheritedEnhanceLevel > 0 {
			if template := s.gameData.EquipmentTemplateByID(id); template != nil {
				rule.EnhanceLevel = min(run.InheritedEnhanceLevel, int(template.EnhanceMaxLevel))
				if rule.EnhanceLevel > 0 {
					rule.EnhancedBase = make(map[string]float64)
					for key, value := range template.BaseAttrs {
						if strings.TrimSpace(key) != "" {
							rule.EnhancedBase[strings.TrimSpace(key)] = value
						}
					}
					if s.enhanceCache != nil {
						for key, value := range s.enhanceCache.CumulativeAttrs(id, rule.EnhanceLevel) {
							if strings.TrimSpace(key) != "" {
								rule.EnhancedBase[strings.TrimSpace(key)] += value
							}
						}
					}
				}
			}
		}
		state.Grants[id] = rule
	}
	var envelope queueRuntimeEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.ActionID == "" {
		envelope.ActionID = item.ActionID
	}
	envelope.OrdinarySettlement = state
	return json.Marshal(envelope)
}

func parseOrdinaryRuntime(raw []byte) parsedGenericExecutorRuntime {
	var envelope queueRuntimeEnvelope
	if json.Unmarshal(raw, &envelope) != nil {
		return parsedGenericExecutorRuntime{}
	}
	var runtime parsedGenericExecutorRuntime
	var value genericExecutorRuntime
	if json.Unmarshal(envelope.Runtime, &value) != nil {
		return runtime
	}
	runtime.BaseDurationMs, runtime.DurationMs, runtime.ExtraAttrExp = value.BaseDurationMs, value.DurationMs, value.ExtraAttrExp
	return runtime
}

func (s *Store) freezeOrdinaryGrant(id string, reward ActionItemRewardSpec) (ordinaryGrantRule, error) {
	rule := ordinaryGrantRule{Stackable: s.itemStackable(id), Durability: s.defaultMaxDurability(id), DefaultAffixes: s.templateDefaultAffixSlots(id), Definitions: map[string]catalog.AffixDefinition{}}
	rule.Commodity = !s.itemHasRandomDefaultAffixGrant(id)
	for _, slot := range rule.DefaultAffixes {
		if def := s.gameData.AffixDefinitionByID(slot.AffixID); def != nil {
			rule.Definitions[slot.AffixID] = *def
		}
	}
	if len(rule.DefaultAffixes) > 0 {
		rule.Stackable = false
	}
	if reward.GrantPolicy != InventoryGrantPolicyRollDefaultAffixes {
		return rule, nil
	}
	rule.Stackable = false
	template := s.gameData.EquipmentTemplateByID(id)
	if template == nil || len(template.DefaultAffixSlots) == 0 || template.AffixSlotCount <= 0 {
		return rule, ErrAffixInvalidCarrier
	}
	weights := normalizedPositiveWeights(reward.DefaultAffixRarityWeights)
	for _, slot := range template.DefaultAffixSlots {
		def := s.gameData.AffixDefinitionByID(slot.AffixID)
		if def == nil || !def.Active || slot.SlotIndex < 0 || int64(slot.SlotIndex) >= template.AffixSlotCount {
			return rule, ErrAffixInvalidCarrier
		}
		roll := ordinaryAffixRoll{Slot: slot.SlotIndex, Definition: *def}
		rule.Definitions[def.ID] = *def
		for rarity, weight := range weights {
			if affixSupportsRarity(*def, rarity) {
				roll.Rarities = append(roll.Rarities, ordinaryRarityWeight{rarity, weight})
			}
		}
		sort.Slice(roll.Rarities, func(i, j int) bool { return roll.Rarities[i].ID < roll.Rarities[j].ID })
		if len(roll.Rarities) == 0 {
			return rule, ErrAffixEnchantNoRarityCandidate
		}
		rule.Rolls = append(rule.Rolls, roll)
	}
	return rule, nil
}

func (rule ordinaryGrantRule) AffixDefinitionByID(id string) *catalog.AffixDefinition {
	def, ok := rule.Definitions[id]
	if !ok {
		return nil
	}
	return &def
}

func (rule ordinaryGrantRule) ItemTraitByItemID(string) *catalog.ItemTraitDefinition { return nil }

func (rule ordinaryGrantRule) enhancedSnapshot(affixes []EquipmentAffixSlot) (json.RawMessage, error) {
	base := make(map[string]float64, len(rule.EnhancedBase))
	for key, value := range rule.EnhancedBase {
		base[key] = value
	}
	for _, slot := range FilterEffectiveAffixSlotsByLimit(affixes, rule, nil) {
		for key, value := range slot.AttrMods {
			if strings.TrimSpace(key) != "" {
				base[strings.TrimSpace(key)] += value
			}
		}
	}
	return json.Marshal(base)
}

func (rule ordinaryGrantRule) affixes(rng *randv2.Rand, rolled bool) []EquipmentAffixSlot {
	if !rolled || len(rule.Rolls) == 0 {
		return rule.DefaultAffixes
	}
	out := make([]EquipmentAffixSlot, 0, len(rule.Rolls))
	for _, roll := range rule.Rolls {
		total := 0.0
		for _, rarity := range roll.Rarities {
			total += rarity.Weight
		}
		needle, sum := rng.Float64()*total, 0.0
		id := roll.Rarities[len(roll.Rarities)-1].ID
		for _, rarity := range roll.Rarities {
			sum += rarity.Weight
			if needle <= sum {
				id = rarity.ID
				break
			}
		}
		out = append(out, buildEquipmentAffixSlot(roll.Definition, id, rng.Float64(), roll.Slot))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SlotIndex < out[j].SlotIndex })
	return out
}

func validateOrdinaryReward(reward ActionItemRewardSpec) error {
	if reward.Percent != nil && (math.IsNaN(*reward.Percent) || math.IsInf(*reward.Percent, 0)) {
		return ErrBadQuantity
	}
	if reward.Count != nil && *reward.Count < 0 {
		return ErrBadQuantity
	}
	if reward.Range != nil && (reward.Range.Min < 0 || reward.Range.Max < reward.Range.Min || reward.Range.Max == math.MaxInt64) {
		return ErrBadQuantity
	}
	if reward.GrantPolicy != "" && reward.GrantPolicy != InventoryGrantPolicyRollDefaultAffixes {
		return errors.New("unsupported ordinary grant policy")
	}
	return nil
}

func ordinaryRewardQuantity(random *actionsettlement.Random, id string, ordinal int, reward ActionItemRewardSpec) int64 {
	p := 1.0
	if reward.Percent != nil {
		p = *reward.Percent
	}
	if !random.Hit("reward:"+id, int64(ordinal), p) {
		return 0
	}
	if reward.Count != nil {
		return *reward.Count
	}
	if reward.Range != nil {
		rng := randv2.New(random.Source("quantity:"+id, int64(ordinal)))
		return reward.Range.Min + rng.Int64N(reward.Range.Max-reward.Range.Min+1)
	}
	return 0
}

func ordinaryEffectStream(grantID int64, kind string) string {
	return "effect:" + strconv.FormatInt(grantID, 10) + ":" + kind
}
