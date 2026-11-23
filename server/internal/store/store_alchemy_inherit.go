package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

const alchemyInheritReservedState = "alchemy_inherit_reserved"

const (
	inheritDecisionInherit  = "inherit"
	inheritDecisionSuppress = "suppress"
	inheritDecisionIgnore   = "ignore"
)

type alchemyInheritArgs struct {
	FatherInstanceID  int64                      `json:"fatherInstanceId"`
	FatherAffixIndex  int                        `json:"fatherAffixIndex"`
	MotherInstanceID  int64                      `json:"motherInstanceId"`
	MotherAffixIndex  int                        `json:"motherAffixIndex"`
	CarrierInstanceID int64                      `json:"carrierInstanceId"`
	MediumItemID      string                     `json:"mediumItemId"`
	GeneChoices       []alchemyInheritGeneChoice `json:"geneChoices"`
}

type alchemyInheritGeneChoice struct {
	Parent         string `json:"parent"`
	ComponentIndex int    `json:"componentIndex"`
	SourceKey      string `json:"sourceKey"`
	Decision       string `json:"decision"`
}

type alchemyInheritComponent struct {
	SourceKind               string                       `json:"sourceKind"`
	SourceKey                string                       `json:"sourceKey"`
	SourceType               string                       `json:"sourceType"`
	SourceID                 string                       `json:"sourceId"`
	ImmediateParent          string                       `json:"immediateParent,omitempty"`
	ComponentKind            string                       `json:"componentKind,omitempty"`
	ItemID                   string                       `json:"itemId,omitempty"`
	TraitID                  string                       `json:"traitId,omitempty"`
	AffixID                  string                       `json:"affixId,omitempty"`
	AffixNameKey             string                       `json:"affixNameKey,omitempty"`
	ComboRecipeID            string                       `json:"comboRecipeId,omitempty"`
	MutationID               string                       `json:"mutationId,omitempty"`
	Family                   string                       `json:"family,omitempty"`
	Attr                     string                       `json:"attr,omitempty"`
	EffectType               string                       `json:"effectType,omitempty"`
	EffectParams             map[string]any               `json:"effectParams,omitempty"`
	Quantity                 int64                        `json:"quantity,omitempty"`
	ValueRange               catalog.AlchemyValueRange    `json:"valueRange"`
	CurrentValue             float64                      `json:"currentValue"`
	MaxEffectivePerEquipment int64                        `json:"maxEffectivePerEquipment"`
	MaxEffectivePerBody      int64                        `json:"maxEffectivePerBody"`
	OperationEffects         *alchemyOperationEffectStamp `json:"operationEffects,omitempty"`
	FusionKind               string                       `json:"fusionKind,omitempty"`
	RetentionReason          string                       `json:"retentionReason,omitempty"`
	SourceKeys               []string                     `json:"sourceKeys,omitempty"`
	SourceItems              []string                     `json:"sourceItems,omitempty"`
	Parents                  []string                     `json:"parents,omitempty"`
	Ancestry                 []alchemyInheritAncestry     `json:"ancestry,omitempty"`
}

type alchemyInheritAncestry struct {
	Parent         string  `json:"parent"`
	ComponentIndex int     `json:"componentIndex"`
	SourceKey      string  `json:"sourceKey"`
	SourceType     string  `json:"sourceType,omitempty"`
	SourceID       string  `json:"sourceId,omitempty"`
	ComponentKind  string  `json:"componentKind,omitempty"`
	ItemID         string  `json:"itemId,omitempty"`
	TraitID        string  `json:"traitId,omitempty"`
	AffixID        string  `json:"affixId,omitempty"`
	AffixNameKey   string  `json:"affixNameKey,omitempty"`
	ComboRecipeID  string  `json:"comboRecipeId,omitempty"`
	MutationID     string  `json:"mutationId,omitempty"`
	Attr           string  `json:"attr,omitempty"`
	CurrentValue   float64 `json:"currentValue"`
	Decision       string  `json:"decision,omitempty"`
}

type inheritParentAffix struct {
	Instance  LockedItemInstance
	Slot      EquipmentAffixSlot
	Kind      string
	Family    string
	ParentTag string
}

type inheritSelectableGene struct {
	Parent         string
	ComponentIndex int
	Component      alchemyInheritComponent
	Decision       string
}

type inheritRetainedGene struct {
	Gene            inheritSelectableGene
	Component       alchemyInheritComponent
	RetentionReason string
}

type alchemyInheritDecisionTrace struct {
	Parent           string  `json:"parent"`
	ComponentIndex   int     `json:"componentIndex"`
	SourceKey        string  `json:"sourceKey"`
	Attr             string  `json:"attr,omitempty"`
	Decision         string  `json:"decision"`
	Outcome          string  `json:"outcome"`
	Value            float64 `json:"value,omitempty"`
	IgnoreKeepChance float64 `json:"ignoreKeepChance,omitempty"`
}

type alchemyInheritFusionTrace struct {
	Attr          string   `json:"attr"`
	FusionKind    string   `json:"fusionKind"`
	PrimarySource string   `json:"primarySource"`
	SourceKeys    []string `json:"sourceKeys"`
	Value         float64  `json:"value"`
}

type alchemyInheritSummary struct {
	Decisions []alchemyInheritDecisionTrace `json:"decisions"`
	Fusions   []alchemyInheritFusionTrace   `json:"fusions"`
}

type resolvedAlchemyInherit struct {
	Kind            string
	EffectType      string
	EffectParams    map[string]any
	AttrMods        map[string]float64
	Value           float64
	RarityID        string
	DisplayNameKey  string
	DisplayNameArgs map[string]any
	Components      []alchemyInheritComponent
	TraitIDs        []string
	ComboIDs        []string
	Failed          bool
	Mutated         bool
	MutationType    string
}

type alchemyInheritResult struct {
	Costs                   map[string]int64   `json:"costs"`
	Rewards                 map[string]int64   `json:"rewards"`
	TargetCarrierInstanceID int64              `json:"targetCarrierInstanceId,omitempty"`
	Affix                   EquipmentAffixSlot `json:"affix,omitempty"`
}

type AlchemyInheritImmediateResult struct {
	Costs                   map[string]int64   `json:"costs"`
	Rewards                 map[string]int64   `json:"rewards"`
	TargetCarrierInstanceID int64              `json:"targetCarrierInstanceId"`
	Affix                   EquipmentAffixSlot `json:"affix"`
}

func (s *Store) AlchemyInheritImmediate(ctx context.Context, userID int64, raw json.RawMessage) (AlchemyInheritImmediateResult, error) {
	executor := newAlchemyExecutor(s, "alchemy.inherit")
	normalized, err := executor.ValidateAndNormalizeArgs(raw)
	if err != nil {
		return AlchemyInheritImmediateResult{}, err
	}
	args, ok := normalized.(alchemyInheritArgs)
	if !ok {
		return AlchemyInheritImmediateResult{}, ErrAlchemyArgsInvalid
	}
	item := QueueRuntimeItem{
		ActionID:   "alchemy.inherit",
		AuditPhase: "instant",
		Spec:       ActionSpec{ID: "alchemy.inherit"},
		TotalCount: 1,
		DoneCount:  0,
	}
	var result alchemyInheritResult
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		_, _, _, medium, err := s.validateAlchemyInheritSetupTx(ctx, tx, userID, args, false)
		if err != nil {
			return err
		}
		if err := s.validateAlchemyInheritMediumLevelForItemTx(ctx, tx, userID, item, medium); err != nil {
			return err
		}
		if err := s.reserveAlchemyInheritInstancesTx(ctx, tx, userID, args); err != nil {
			return err
		}
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		next, err := s.alchemyInheritSettleTx(ctx, tx, userID, args, rng, item, false)
		if err != nil {
			return err
		}
		result = next
		return nil
	})
	if err != nil {
		return AlchemyInheritImmediateResult{}, err
	}
	return AlchemyInheritImmediateResult{
		Costs:                   result.Costs,
		Rewards:                 result.Rewards,
		TargetCarrierInstanceID: result.TargetCarrierInstanceID,
		Affix:                   result.Affix,
	}, nil
}

func (e *alchemyExecutor) validateAndNormalizeInheritArgs(raw json.RawMessage) (alchemyInheritArgs, error) {
	var args alchemyInheritArgs
	if err := decodeAlchemyArgsStrict(raw, &args); err != nil {
		return args, ErrAlchemyArgsInvalid
	}
	args.MediumItemID = strings.TrimSpace(args.MediumItemID)
	if args.MediumItemID == "" {
		return args, ErrAlchemyMediumUnknown
	}
	choices, _, _, err := normalizeInheritGeneChoices(args.GeneChoices)
	if err != nil {
		return args, err
	}
	args.GeneChoices = choices
	if args.FatherInstanceID <= 0 || args.MotherInstanceID <= 0 || args.CarrierInstanceID <= 0 {
		return args, ErrAlchemyArgsInvalid
	}
	if args.FatherInstanceID == args.MotherInstanceID ||
		args.FatherInstanceID == args.CarrierInstanceID ||
		args.MotherInstanceID == args.CarrierInstanceID {
		return args, ErrAlchemyArgsInvalid
	}
	if args.FatherAffixIndex < 0 || args.MotherAffixIndex < 0 {
		return args, ErrAlchemyArgsInvalid
	}
	if e.store != nil && args.MediumItemID != "" {
		medium := e.store.gameData.AlchemyInheritMediumByItemID(args.MediumItemID)
		if medium == nil {
			return args, ErrAlchemyMediumUnknown
		}
	}
	return args, nil
}

func normalizeInheritGeneChoices(values []alchemyInheritGeneChoice) ([]alchemyInheritGeneChoice, int, int, error) {
	seen := map[string]bool{}
	out := make([]alchemyInheritGeneChoice, 0, len(values))
	inheritCount := 0
	suppressCount := 0
	for _, value := range values {
		choice := alchemyInheritGeneChoice{
			Parent:         strings.TrimSpace(value.Parent),
			ComponentIndex: value.ComponentIndex,
			SourceKey:      strings.TrimSpace(value.SourceKey),
			Decision:       strings.TrimSpace(value.Decision),
		}
		if choice.Decision == "" {
			choice.Decision = inheritDecisionIgnore
		}
		if choice.Parent != "father" && choice.Parent != "mother" {
			return nil, 0, 0, ErrAlchemyArgsInvalid
		}
		if choice.ComponentIndex < 0 || choice.SourceKey == "" {
			return nil, 0, 0, ErrAlchemyArgsInvalid
		}
		if choice.Decision != inheritDecisionInherit && choice.Decision != inheritDecisionSuppress && choice.Decision != inheritDecisionIgnore {
			return nil, 0, 0, ErrAlchemyArgsInvalid
		}
		key := inheritChoiceKey(choice.Parent, choice.ComponentIndex, choice.SourceKey)
		if seen[key] {
			return nil, 0, 0, ErrAlchemyArgsInvalid
		}
		seen[key] = true
		if choice.Decision == inheritDecisionInherit {
			inheritCount++
		}
		if choice.Decision == inheritDecisionSuppress {
			suppressCount++
		}
		out = append(out, choice)
	}
	sort.Slice(out, func(i, j int) bool {
		left := inheritChoiceKey(out[i].Parent, out[i].ComponentIndex, out[i].SourceKey)
		right := inheritChoiceKey(out[j].Parent, out[j].ComponentIndex, out[j].SourceKey)
		return left < right
	})
	return out, inheritCount, suppressCount, nil
}

func inheritChoiceKey(parent string, componentIndex int, sourceKey string) string {
	return fmt.Sprintf("%s:%d:%s", parent, componentIndex, sourceKey)
}

func (s *Store) alchemyInheritPerRunCosts(args alchemyInheritArgs, medium *catalog.AlchemyInheritMedium) map[string]int64 {
	costs := map[string]int64{}
	if medium != nil && medium.ConsumePerCraft > 0 {
		costs[medium.ItemID] += medium.ConsumePerCraft
	}
	inheritCount := int64(0)
	suppressCount := int64(0)
	for _, choice := range args.GeneChoices {
		switch choice.Decision {
		case inheritDecisionInherit:
			inheritCount++
		case inheritDecisionSuppress:
			suppressCount++
		}
	}
	if medium != nil {
		addInheritCost(costs, medium.Modifiers.InheritCost, inheritCount)
		addInheritCost(costs, medium.Modifiers.SuppressCost, suppressCount)
	}
	for itemID, qty := range costs {
		if itemID == "" || qty <= 0 {
			delete(costs, itemID)
		}
	}
	return costs
}

func addInheritCost(costs map[string]int64, cost catalog.AlchemyInheritCost, count int64) {
	if count <= 0 {
		return
	}
	if cost.Gold > 0 {
		costs[GoldCoinItemID] += cost.Gold * count
	}
	for itemID, qty := range cost.Items {
		itemID = strings.TrimSpace(itemID)
		if itemID != "" && qty > 0 {
			costs[itemID] += qty * count
		}
	}
}

func (s *Store) reserveAlchemyInheritInstancesTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyInheritArgs) error {
	for _, instanceID := range []int64{args.FatherInstanceID, args.MotherInstanceID, args.CarrierInstanceID} {
		if err := s.SetStateTx(ctx, tx, instanceID, userID, "inventory", alchemyInheritReservedState); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) releaseAlchemyInheritReservationsTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyInheritArgs) error {
	for _, instanceID := range []int64{args.FatherInstanceID, args.MotherInstanceID, args.CarrierInstanceID} {
		_ = s.SetStateTx(ctx, tx, instanceID, userID, alchemyInheritReservedState, "inventory")
	}
	return nil
}

func (s *Store) lockInheritParentTx(ctx context.Context, tx pgx.Tx, userID int64, instanceID int64, affixIndex int, parentTag string, requireReserved bool) (inheritParentAffix, error) {
	locked, err := s.LockInstanceTx(ctx, tx, instanceID)
	if err != nil {
		return inheritParentAffix{}, err
	}
	if locked.UserID != userID {
		return inheritParentAffix{}, ErrInstanceNotOwned
	}
	expectedState := "inventory"
	if requireReserved {
		expectedState = alchemyInheritReservedState
	}
	if locked.State != expectedState {
		return inheritParentAffix{}, ErrInvalidInstanceState
	}
	slots := decodeAffixSlots(locked.AffixesRaw)
	if affixIndex < 0 || affixIndex >= len(slots) {
		return inheritParentAffix{}, ErrAffixSlotNotFound
	}
	slot := slots[affixIndex]
	if !s.alchemyInheritParentSlotEligible(slot, locked.TemplateItem) {
		return inheritParentAffix{}, ErrAlchemyArgsInvalid
	}
	kind := s.alchemyInheritSlotKind(slot)
	if kind == "" {
		return inheritParentAffix{}, ErrAlchemyArgsInvalid
	}
	return inheritParentAffix{Instance: locked, Slot: slot, Kind: kind, Family: s.inheritSlotFamily(slot), ParentTag: parentTag}, nil
}

func (s *Store) alchemyInheritParentSlotEligible(slot EquipmentAffixSlot, templateItemID string) bool {
	affixID := strings.TrimSpace(slot.AffixID)
	if affixID == "" {
		return false
	}
	if strings.HasPrefix(affixID, "custom:") {
		sourceKind := strings.TrimSpace(slot.SourceKind)
		if sourceKind != "craft" && sourceKind != "inherit" {
			return false
		}
		if slot.SourceKind == "inherit_orphan" || boolFromAny(slot.EffectParams["inheritOrphan"]) {
			return false
		}
		if s.alchemyInheritSlotKind(slot) == "" {
			return false
		}
		return len(s.extractAlchemyInheritComponentsFromParentSlot(slot, "")) > 0
	}
	if rule := s.gameData.AffixCarrierRuleByItemID(templateItemID); rule == nil || !rule.Active {
		return false
	}
	if s.alchemyInheritSlotKind(slot) == "" {
		return false
	}
	return len(s.extractAlchemyInheritComponentsFromParentSlot(slot, "")) > 0
}

func (s *Store) alchemyInheritSlotKind(slot EquipmentAffixSlot) string {
	if strings.HasPrefix(strings.TrimSpace(slot.AffixID), "custom:") {
		return strings.TrimSpace(fmt.Sprint(slot.EffectParams["alchemyKind"]))
	}
	def := s.gameData.AffixDefinitionByID(strings.TrimSpace(slot.AffixID))
	if def == nil || !def.Active {
		return ""
	}
	return alchemyInheritKindFromAffixType(def.Type)
}

func alchemyInheritKindFromAffixType(affixType string) string {
	switch strings.TrimSpace(strings.ToLower(affixType)) {
	case "battle", "combat":
		return "combat"
	case "work", "life":
		return "life"
	default:
		return ""
	}
}

func boolFromAny(raw any) bool {
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		normalized := strings.TrimSpace(strings.ToLower(value))
		return normalized == "true" || normalized == "1" || normalized == "yes"
	case float64:
		return value != 0
	case int:
		return value != 0
	case int64:
		return value != 0
	default:
		return false
	}
}

func (s *Store) inheritSlotFamily(slot EquipmentAffixSlot) string {
	families := map[string]int{}
	for _, component := range s.extractAlchemyInheritComponentsFromParentSlot(slot, "") {
		if component.Family != "" {
			families[component.Family]++
		}
	}
	return majorityFamily(families)
}

func (s *Store) validateAlchemyInheritSetupTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyInheritArgs, requireReserved bool) (inheritParentAffix, inheritParentAffix, LockedItemInstance, *catalog.AlchemyInheritMedium, error) {
	if err := s.lockAlchemyInheritInstancesInOrderTx(ctx, tx, args); err != nil {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, err
	}
	father, err := s.lockInheritParentTx(ctx, tx, userID, args.FatherInstanceID, args.FatherAffixIndex, "father", requireReserved)
	if err != nil {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, err
	}
	mother, err := s.lockInheritParentTx(ctx, tx, userID, args.MotherInstanceID, args.MotherAffixIndex, "mother", requireReserved)
	if err != nil {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, err
	}
	if father.Kind != mother.Kind {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAlchemyKindMismatch
	}
	carrier, err := s.LockInstanceTx(ctx, tx, args.CarrierInstanceID)
	if err != nil {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, err
	}
	if carrier.UserID != userID {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrInstanceNotOwned
	}
	expectedState := "inventory"
	if requireReserved {
		expectedState = alchemyInheritReservedState
	}
	if carrier.State != expectedState {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrInvalidInstanceState
	}
	if !affixesJSONEmptyRaw(carrier.AffixesRaw) || len(decodeAffixSlots(carrier.AffixesRaw)) > 0 {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAlchemyCarrierOccupied
	}
	rule := s.gameData.AffixCarrierRuleByItemID(carrier.TemplateItem)
	if rule == nil || !rule.Active || rule.MaxCustomAffixes <= 0 {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAffixInvalidCarrier
	}
	if args.MediumItemID == "" {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAlchemyMediumUnknown
	}
	medium := s.gameData.AlchemyInheritMediumByItemID(args.MediumItemID)
	if medium == nil {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAlchemyMediumUnknown
	}
	rarityID := s.inheritOutputRarity(father.Slot.RarityID, mother.Slot.RarityID, medium, false)
	if !carrierRuleAllowsRarity(rule, rarityID) {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAffixRarityNotAllowed
	}
	if rule.AllowedTraitKind != "" && rule.AllowedTraitKind != "both" && rule.AllowedTraitKind != father.Kind {
		return inheritParentAffix{}, inheritParentAffix{}, LockedItemInstance{}, nil, ErrAlchemyKindMismatch
	}
	return father, mother, carrier, medium, nil
}

func validateAlchemyInheritMediumLevel(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) error {
	if medium == nil {
		return ErrAlchemyMediumUnknown
	}
	if medium.MinAlchemyLevel > 0 && derived.Level < float64(medium.MinAlchemyLevel) {
		return ErrAlchemyLevelInsufficient
	}
	return nil
}

func (s *Store) validateAlchemyInheritMediumLevelForItemTx(ctx context.Context, tx pgx.Tx, userID int64, item QueueRuntimeItem, medium *catalog.AlchemyInheritMedium) error {
	derived, err := s.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		return err
	}
	return validateAlchemyInheritMediumLevel(medium, derived)
}

func (s *Store) lockAlchemyInheritInstancesInOrderTx(ctx context.Context, tx pgx.Tx, args alchemyInheritArgs) error {
	ids := []int64{args.FatherInstanceID, args.MotherInstanceID, args.CarrierInstanceID}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, instanceID := range ids {
		if _, err := s.LockInstanceTx(ctx, tx, instanceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) alchemyInheritSettleTx(ctx context.Context, tx pgx.Tx, userID int64, args alchemyInheritArgs, rng *rand.Rand, item QueueRuntimeItem, prepaid bool) (alchemyInheritResult, error) {
	father, mother, carrier, medium, err := s.validateAlchemyInheritSetupTx(ctx, tx, userID, args, true)
	if err != nil {
		_ = s.releaseAlchemyInheritReservationsTx(ctx, tx, userID, args)
		return alchemyInheritResult{}, err
	}
	derived, err := s.resolveAlchemyDerivedForItemTx(ctx, tx, userID, item)
	if err != nil {
		_ = s.releaseAlchemyInheritReservationsTx(ctx, tx, userID, args)
		return alchemyInheritResult{}, err
	}
	if err := validateAlchemyInheritMediumLevel(medium, derived); err != nil {
		_ = s.releaseAlchemyInheritReservationsTx(ctx, tx, userID, args)
		return alchemyInheritResult{}, err
	}
	resolved, err := s.resolveAlchemyInherit(father, mother, medium, args, derived, rng)
	if err != nil {
		_ = s.releaseAlchemyInheritReservationsTx(ctx, tx, userID, args)
		return alchemyInheritResult{}, err
	}
	slot := alchemyInheritSlot(resolved)
	targetCarrierID := carrier.InstanceID
	if carrier.Quantity > 1 {
		targetCarrierID, err = s.SplitOneFromStackTx(ctx, tx, carrier.InstanceID, userID)
		if err != nil {
			return alchemyInheritResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_item_instances
		SET affixes = $2::jsonb, state = 'inventory', updated_at = now()
		WHERE instance_id = $1 AND user_id = $3 AND state = $4
	`, targetCarrierID, string(encodeAffixSlots([]EquipmentAffixSlot{slot})), userID, alchemyInheritReservedState); err != nil {
		return alchemyInheritResult{}, err
	}
	if targetCarrierID != carrier.InstanceID {
		if err := s.SetStateTx(ctx, tx, carrier.InstanceID, userID, alchemyInheritReservedState, "inventory"); err != nil {
			return alchemyInheritResult{}, err
		}
	}
	if err := s.refreshInstanceBaseSnapshotTx(ctx, tx, targetCarrierID, userID); err != nil {
		return alchemyInheritResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM player_item_instances
		WHERE user_id = $1 AND state = $2 AND instance_id IN ($3, $4)
	`, userID, alchemyInheritReservedState, father.Instance.InstanceID, mother.Instance.InstanceID); err != nil {
		return alchemyInheritResult{}, err
	}
	for _, traitID := range resolved.TraitIDs {
		_ = s.markAlchemyDiscoveredTx(ctx, tx, userID, "trait", traitID)
	}
	for _, comboID := range resolved.ComboIDs {
		_ = s.markAlchemyDiscoveredTx(ctx, tx, userID, "combo", comboID)
	}
	costs := s.alchemyInheritPerRunCosts(args, medium)
	if !prepaid {
		for itemID, qty := range costs {
			if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, qty, alchemyLedgerMeta(item, "inventory", "alchemy_inherit", InventoryAuditDirectionConsume, "alchemy_inherit_cost", map[string]any{"itemId": itemID})); err != nil {
				return alchemyInheritResult{}, err
			}
		}
	}
	return alchemyInheritResult{Costs: costs, Rewards: map[string]int64{}, TargetCarrierInstanceID: targetCarrierID, Affix: slot}, nil
}

func alchemyInheritSlot(resolved resolvedAlchemyInherit) EquipmentAffixSlot {
	return EquipmentAffixSlot{
		SlotIndex:             0,
		AffixID:               encodeCustomAffixID(resolved.EffectType, resolved.EffectParams, resolved.Value, 3),
		RarityID:              resolved.RarityID,
		SourceKind:            inheritOutputSourceKind(resolved.EffectParams),
		AlchemyRulesetVersion: AlchemyCurrentRulesetVersion,
		DisplayNameKey:        resolved.DisplayNameKey,
		DisplayNameArgs:       resolved.DisplayNameArgs,
		ValuePrecision:        3,
		AttrMods:              resolved.AttrMods,
		EffectType:            resolved.EffectType,
		EffectParams:          resolved.EffectParams,
	}
}

func inheritOutputSourceKind(params map[string]any) string {
	if boolFromAny(params["inheritOrphan"]) {
		return "inherit_orphan"
	}
	return "inherit"
}

func (s *Store) resolveAlchemyInherit(father, mother inheritParentAffix, medium *catalog.AlchemyInheritMedium, args alchemyInheritArgs, derived alchemyDerived, rng *rand.Rand) (resolvedAlchemyInherit, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	rules := s.effectiveAlchemyInheritRules(medium)
	selectable := s.buildInheritSelectableGenes(father, mother)
	if len(selectable) == 0 {
		return resolvedAlchemyInherit{}, ErrAlchemyArgsInvalid
	}
	choices, err := applyInheritChoices(selectable, args.GeneChoices, s.effectiveAlchemyInheritInheritSlots(medium, derived), s.effectiveAlchemyInheritSuppressSlots(medium, derived))
	if err != nil {
		return resolvedAlchemyInherit{}, err
	}
	retained := make([]inheritRetainedGene, 0, len(selectable))
	summary := alchemyInheritSummary{Decisions: make([]alchemyInheritDecisionTrace, 0, len(selectable))}
	for _, gene := range choices {
		component := gene.Component
		fillInheritComponentSourceFields(&component)
		trace := alchemyInheritDecisionTrace{
			Parent:         gene.Parent,
			ComponentIndex: gene.ComponentIndex,
			SourceKey:      component.SourceKey,
			Attr:           component.Attr,
			Decision:       gene.Decision,
		}
		switch gene.Decision {
		case inheritDecisionSuppress:
			trace.Outcome = "suppressed"
		case inheritDecisionInherit:
			component.RetentionReason = inheritDecisionInherit
			retained = append(retained, inheritRetainedGene{Gene: gene, Component: component, RetentionReason: inheritDecisionInherit})
			trace.Outcome = "retained"
			trace.Value = component.CurrentValue
		default:
			ignoreChance := inheritAdjustedIgnoreKeepChanceForComponent(&rules, derived, component)
			trace.IgnoreKeepChance = ignoreChance
			if rng.Float64() < ignoreChance {
				component.RetentionReason = inheritDecisionIgnore
				retained = append(retained, inheritRetainedGene{Gene: gene, Component: component, RetentionReason: inheritDecisionIgnore})
				trace.Outcome = "retained_by_ignore"
				trace.Value = component.CurrentValue
			} else {
				trace.Outcome = "discarded_by_ignore"
			}
		}
		summary.Decisions = append(summary.Decisions, trace)
	}
	if len(retained) == 0 {
		return resolvedAlchemyInherit{}, ErrAlchemyArgsInvalid
	}
	components, fusions := fuseRetainedInheritGenes(retained, &rules, derived, rng, s.alchemyInheritNumericCapMultiplier())
	summary.Fusions = fusions
	attrMods := inheritAttrModsFromComponents(components)
	orphan := false
	complexParams := map[string]any{}
	for _, component := range components {
		if component.SourceKind == "ordinary_affix" || component.SourceType == "affix" {
			orphan = true
		}
		mergeInheritedEffectParams(complexParams, component.EffectParams)
	}
	mutated := false
	mutationType := ""
	if rng.Float64() < rules.Modifiers.MutationChance {
		mutated, mutationType = s.applyInheritMutation(&components, attrMods, father.Kind, &rules, derived, rng)
		attrMods = inheritAttrModsFromComponents(components)
	}
	componentFamilies := map[string]int{}
	traitIDs := map[string]bool{}
	comboIDs := map[string]bool{}
	for _, component := range components {
		if component.Family != "" {
			componentFamilies[component.Family]++
		}
		if component.TraitID != "" {
			traitIDs[component.TraitID] = true
		}
		if component.ComboRecipeID != "" {
			comboIDs[component.ComboRecipeID] = true
		}
	}
	displayKey, displayArgs := s.computeAlchemyInheritDisplayName(father, mother, components)
	params := map[string]any{
		"attrMods":              attrMods,
		"alchemyKind":           father.Kind,
		"inheritSourceKind":     "inherit",
		"inheritFamily":         majorityFamily(componentFamilies),
		"alchemyComponents":     components,
		"alchemyInheritSummary": summary,
	}
	for key, value := range complexParams {
		params[key] = value
	}
	if orphan {
		params["inheritOrphan"] = true
		params["inheritOrphanReason"] = "ordinary_affix_source"
		params["inheritSourceKind"] = "inherit_orphan"
	}
	if mutated {
		params["inheritMutationType"] = mutationType
	}
	return resolvedAlchemyInherit{
		Kind:            father.Kind,
		EffectType:      "add_stat",
		EffectParams:    params,
		AttrMods:        attrMods,
		Value:           stableValueSum(attrMods),
		RarityID:        s.inheritOutputRarity(father.Slot.RarityID, mother.Slot.RarityID, medium, false),
		DisplayNameKey:  displayKey,
		DisplayNameArgs: displayArgs,
		Components:      components,
		TraitIDs:        sortedStringSet(traitIDs),
		ComboIDs:        sortedStringSet(comboIDs),
		Failed:          false,
		Mutated:         mutated,
		MutationType:    mutationType,
	}, nil
}

func (s *Store) computeAlchemyInheritDisplayName(father, mother inheritParentAffix, components []alchemyInheritComponent) (string, map[string]any) {
	materials := map[string]int64{}
	ordinaryNameKeys := map[string]bool{}
	comboNameKeys := map[string]bool{}
	for _, component := range components {
		collectInheritDisplayNameParts(s.gameData, component, materials, ordinaryNameKeys, comboNameKeys)
	}
	materialKey, materialArgs := s.computeAlchemyDisplayName(materials, "")
	if len(materials) > 0 && len(ordinaryNameKeys) > 0 {
		args := cloneStringAnyMap(materialArgs)
		args["affixNameKeys"] = sortedStringSet(ordinaryNameKeys)
		return "alchemy.affix.template.customWithAffixes", args
	}
	if len(materials) > 0 {
		return materialKey, materialArgs
	}
	if len(ordinaryNameKeys) > 0 {
		return "alchemy.affix.template.affixes", map[string]any{"affixNameKeys": sortedStringSet(ordinaryNameKeys)}
	}
	if len(comboNameKeys) > 0 {
		return "alchemy.affix.template.affixes", map[string]any{"affixNameKeys": sortedStringSet(comboNameKeys)}
	}
	if strings.TrimSpace(father.Slot.DisplayNameKey) != "" {
		return father.Slot.DisplayNameKey, father.Slot.DisplayNameArgs
	}
	if strings.TrimSpace(mother.Slot.DisplayNameKey) != "" {
		return mother.Slot.DisplayNameKey, mother.Slot.DisplayNameArgs
	}
	parentKeys := map[string]bool{}
	collectOrdinaryAffixNameKey(s.gameData, father.Slot.AffixID, "", parentKeys)
	collectOrdinaryAffixNameKey(s.gameData, mother.Slot.AffixID, "", parentKeys)
	if len(parentKeys) > 0 {
		return "alchemy.affix.template.affixes", map[string]any{"affixNameKeys": sortedStringSet(parentKeys)}
	}
	return materialKey, materialArgs
}

func collectInheritDisplayNameParts(data catalog.Data, component alchemyInheritComponent, materials map[string]int64, ordinaryNameKeys map[string]bool, comboNameKeys map[string]bool) {
	addMaterialDisplayItem(data, component.ItemID, materials)
	for _, itemID := range component.SourceItems {
		addMaterialDisplayItem(data, itemID, materials)
	}
	collectOrdinaryAffixNameKey(data, component.AffixID, component.AffixNameKey, ordinaryNameKeys)
	collectComboNameKey(data, component.ComboRecipeID, comboNameKeys)
	for _, row := range component.Ancestry {
		addMaterialDisplayItem(data, row.ItemID, materials)
		collectOrdinaryAffixNameKey(data, row.AffixID, row.AffixNameKey, ordinaryNameKeys)
		collectComboNameKey(data, row.ComboRecipeID, comboNameKeys)
	}
}

func addMaterialDisplayItem(data catalog.Data, itemID string, materials map[string]int64) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return
	}
	if data.ItemTraitByItemID(itemID) == nil || data.ItemByID(itemID) == nil || data.ItemAlchemyNameAffix(itemID) == nil {
		return
	}
	materials[itemID] = 1
}

func collectOrdinaryAffixNameKey(data catalog.Data, affixID string, nameKey string, out map[string]bool) {
	affixID = strings.TrimSpace(affixID)
	if affixID == "" || strings.HasPrefix(affixID, "custom:") {
		return
	}
	nameKey = strings.TrimSpace(nameKey)
	if nameKey == "" {
		if def := data.AffixDefinitionByID(affixID); def != nil {
			nameKey = strings.TrimSpace(def.NameKey)
		}
	}
	if nameKey != "" {
		out[nameKey] = true
	}
}

func collectComboNameKey(data catalog.Data, comboRecipeID string, out map[string]bool) {
	comboRecipeID = strings.TrimSpace(comboRecipeID)
	if comboRecipeID == "" {
		return
	}
	if recipe := data.AlchemyComboRecipeByID(comboRecipeID); recipe != nil && strings.TrimSpace(recipe.NameKey) != "" {
		out[strings.TrimSpace(recipe.NameKey)] = true
	}
}

func cloneStringAnyMap(values map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range values {
		out[key] = value
	}
	return out
}

func (s *Store) buildInheritSelectableGenes(father, mother inheritParentAffix) []inheritSelectableGene {
	out := []inheritSelectableGene{}
	addComponents := func(parent inheritParentAffix) {
		components := s.extractAlchemyInheritComponentsFromParentSlot(parent.Slot, parent.ParentTag)
		for idx, component := range components {
			if component.SourceKey == "" || (component.Attr == "" && component.ComponentKind != "effect") {
				continue
			}
			fillInheritComponentSourceFields(&component)
			component.ImmediateParent = parent.ParentTag
			out = append(out, inheritSelectableGene{
				Parent:         parent.ParentTag,
				ComponentIndex: idx,
				Component:      component,
				Decision:       inheritDecisionIgnore,
			})
		}
	}
	addComponents(father)
	addComponents(mother)
	sort.SliceStable(out, func(i, j int) bool {
		return inheritChoiceKey(out[i].Parent, out[i].ComponentIndex, out[i].Component.SourceKey) <
			inheritChoiceKey(out[j].Parent, out[j].ComponentIndex, out[j].Component.SourceKey)
	})
	return out
}

func applyInheritChoices(selectable []inheritSelectableGene, choices []alchemyInheritGeneChoice, inheritSlotLimit int, suppressSlotLimit int) ([]inheritSelectableGene, error) {
	byKey := map[string]int{}
	for idx, gene := range selectable {
		byKey[inheritChoiceKey(gene.Parent, gene.ComponentIndex, gene.Component.SourceKey)] = idx
	}
	out := make([]inheritSelectableGene, len(selectable))
	copy(out, selectable)
	inheritCount := 0
	suppressCount := 0
	for _, choice := range choices {
		idx, ok := byKey[inheritChoiceKey(choice.Parent, choice.ComponentIndex, choice.SourceKey)]
		if !ok {
			return nil, ErrAlchemyArgsInvalid
		}
		out[idx].Decision = choice.Decision
		if choice.Decision == inheritDecisionInherit {
			inheritCount++
		}
		if choice.Decision == inheritDecisionSuppress {
			suppressCount++
		}
	}
	if inheritCount > inheritSlotLimit || suppressCount > suppressSlotLimit {
		return nil, ErrAlchemyArgsInvalid
	}
	return out, nil
}

func fuseRetainedInheritGenes(retained []inheritRetainedGene, medium *catalog.AlchemyInheritMedium, derived alchemyDerived, rng *rand.Rand, capMultiplier float64) ([]alchemyInheritComponent, []alchemyInheritFusionTrace) {
	attrRetained := make([]inheritRetainedGene, 0, len(retained))
	effectRetained := make([]inheritRetainedGene, 0)
	for _, row := range retained {
		if strings.TrimSpace(row.Component.Attr) == "" || row.Component.ComponentKind == "effect" {
			effectRetained = append(effectRetained, row)
			continue
		}
		attrRetained = append(attrRetained, row)
	}
	byAttr := map[string][]inheritRetainedGene{}
	for _, row := range attrRetained {
		byAttr[row.Component.Attr] = append(byAttr[row.Component.Attr], row)
	}
	attrs := make([]string, 0, len(byAttr))
	for attr := range byAttr {
		attrs = append(attrs, attr)
	}
	sort.Strings(attrs)
	components := []alchemyInheritComponent{}
	traces := []alchemyInheritFusionTrace{}
	for _, attr := range attrs {
		rows := byAttr[attr]
		sort.SliceStable(rows, func(i, j int) bool {
			left := rows[i].Component.SourceKey + rows[i].Gene.Parent
			right := rows[j].Component.SourceKey + rows[j].Gene.Parent
			return left < right
		})
		bySource := map[string][]inheritRetainedGene{}
		sourceKeys := []string{}
		for _, row := range rows {
			key := row.Component.SourceKey
			if _, ok := bySource[key]; !ok {
				sourceKeys = append(sourceKeys, key)
			}
			bySource[key] = append(bySource[key], row)
		}
		sort.Strings(sourceKeys)
		intermediate := make([]alchemyInheritComponent, 0, len(sourceKeys))
		for _, sourceKey := range sourceKeys {
			group := bySource[sourceKey]
			if len(group) == 1 || !retainedGroupHasBothParents(group) {
				for _, row := range group {
					intermediate = append(intermediate, retainedComponentWithoutFusion(row))
				}
				continue
			}
			component, trace := fuseInheritGroup(group, "inbreed_stable", medium, derived, rng, capMultiplier)
			if rng.Float64() < inheritAdjustedInbreedMutationChance(medium, derived) {
				goodChance := inheritAdjustedInbreedGoodChance(medium, derived)
				if rng.Float64() < goodChance {
					component.FusionKind = "inbreed_good"
					component.RetentionReason = "inbreed_mutation_good"
					component.CurrentValue = roundAlchemyValue(applyInbreedGood(component.CurrentValue, medium, rng), 3)
				} else {
					component.FusionKind = "inbreed_bad"
					component.RetentionReason = "inbreed_mutation_bad"
					component.CurrentValue = roundAlchemyValue(applyInbreedBad(component.CurrentValue, medium, derived, rng), 3)
				}
				clampInheritComponentToSourceCap(&component, capMultiplier)
				trace.FusionKind = component.FusionKind
				trace.Value = component.CurrentValue
			}
			intermediate = append(intermediate, component)
			traces = append(traces, trace)
		}
		if len(intermediate) == 1 {
			components = append(components, intermediate[0])
			continue
		}
		group := retainedFromComponents(intermediate)
		if !retainedGroupHasCrossParentDistinctSource(group) {
			components = append(components, intermediate...)
			continue
		}
		component, trace := fuseInheritGroup(group, "hybrid", medium, derived, rng, capMultiplier)
		components = append(components, component)
		traces = append(traces, trace)
	}
	effects, effectTraces := dedupeRetainedInheritEffects(effectRetained, rng)
	components = append(components, effects...)
	traces = append(traces, effectTraces...)
	sort.SliceStable(components, func(i, j int) bool {
		left := strings.Join([]string{components[i].Attr, components[i].SourceKey, components[i].ComponentKind}, "\x00")
		right := strings.Join([]string{components[j].Attr, components[j].SourceKey, components[j].ComponentKind}, "\x00")
		return left < right
	})
	return components, traces
}

func retainedComponentWithoutFusion(row inheritRetainedGene) alchemyInheritComponent {
	component := row.Component
	component.RetentionReason = row.RetentionReason
	component.Ancestry = ancestryFromRetained([]inheritRetainedGene{row})
	component.SourceKeys = uniqueStrings([]string{component.SourceKey})
	component.SourceItems = uniqueComponentItems([]alchemyInheritComponent{component})
	component.Parents = uniqueParentsFromAncestry(component.Ancestry)
	fillInheritComponentSourceFields(&component)
	return component
}

func retainedGroupHasBothParents(group []inheritRetainedGene) bool {
	seen := map[string]bool{}
	for _, row := range group {
		parent := strings.TrimSpace(row.Gene.Parent)
		if parent == "" {
			parent = strings.TrimSpace(row.Component.ImmediateParent)
		}
		seen[parent] = true
	}
	return seen["father"] && seen["mother"]
}

func retainedGroupHasCrossParentDistinctSource(group []inheritRetainedGene) bool {
	fatherSources := map[string]bool{}
	motherSources := map[string]bool{}
	for _, row := range group {
		sourceKey := strings.TrimSpace(row.Component.SourceKey)
		parent := strings.TrimSpace(row.Gene.Parent)
		if parent == "" {
			parent = strings.TrimSpace(row.Component.ImmediateParent)
		}
		switch parent {
		case "father":
			fatherSources[sourceKey] = true
		case "mother":
			motherSources[sourceKey] = true
		}
	}
	for fatherSource := range fatherSources {
		for motherSource := range motherSources {
			if fatherSource != motherSource {
				return true
			}
		}
	}
	return false
}

func dedupeRetainedInheritEffects(retained []inheritRetainedGene, rng *rand.Rand) ([]alchemyInheritComponent, []alchemyInheritFusionTrace) {
	if len(retained) == 0 {
		return nil, nil
	}
	bySource := map[string][]inheritRetainedGene{}
	keys := []string{}
	for _, row := range retained {
		key := strings.TrimSpace(row.Component.SourceKey)
		if key == "" {
			key = fmt.Sprintf("effect:%s:%d", row.Gene.Parent, row.Gene.ComponentIndex)
		}
		if _, ok := bySource[key]; !ok {
			keys = append(keys, key)
		}
		bySource[key] = append(bySource[key], row)
	}
	sort.Strings(keys)
	components := make([]alchemyInheritComponent, 0, len(keys))
	traces := []alchemyInheritFusionTrace{}
	for _, key := range keys {
		group := bySource[key]
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].Gene.Parent < group[j].Gene.Parent
		})
		primaryIdx := 0
		if len(group) > 1 && rng != nil {
			primaryIdx = rng.Intn(len(group))
		}
		component := group[primaryIdx].Component
		component.ComponentKind = "effect"
		component.CurrentValue = 0
		component.ValueRange = catalog.AlchemyValueRange{}
		component.RetentionReason = group[primaryIdx].RetentionReason
		if len(group) > 1 {
			component.FusionKind = "effect_dedupe"
			component.RetentionReason = "effect_dedupe"
		} else if component.FusionKind == "" {
			component.FusionKind = "effect"
		}
		component.Ancestry = ancestryFromRetained(group)
		component.SourceKeys = uniqueComponentSourceKeys(retainedComponentsFromGroup(group))
		component.SourceItems = uniqueComponentItems(retainedComponentsFromGroup(group))
		component.Parents = uniqueParentsFromAncestry(component.Ancestry)
		fillInheritComponentSourceFields(&component)
		components = append(components, component)
		if len(group) > 1 {
			traces = append(traces, alchemyInheritFusionTrace{
				Attr:          "effect",
				FusionKind:    component.FusionKind,
				PrimarySource: component.SourceKey,
				SourceKeys:    component.SourceKeys,
				Value:         0,
			})
		}
	}
	return components, traces
}

func retainedComponentsFromGroup(group []inheritRetainedGene) []alchemyInheritComponent {
	components := make([]alchemyInheritComponent, 0, len(group))
	for _, row := range group {
		components = append(components, row.Component)
	}
	return components
}

func retainedFromComponents(components []alchemyInheritComponent) []inheritRetainedGene {
	out := make([]inheritRetainedGene, 0, len(components))
	for idx, component := range components {
		parent := component.ImmediateParent
		if parent == "" && len(component.Parents) > 0 {
			parent = component.Parents[0]
		}
		out = append(out, inheritRetainedGene{
			Gene: inheritSelectableGene{
				Parent:         parent,
				ComponentIndex: idx,
				Component:      component,
				Decision:       component.RetentionReason,
			},
			Component:       component,
			RetentionReason: component.RetentionReason,
		})
	}
	return out
}

func fuseInheritGroup(group []inheritRetainedGene, fusionKind string, medium *catalog.AlchemyInheritMedium, derived alchemyDerived, rng *rand.Rand, capMultiplier float64) (alchemyInheritComponent, alchemyInheritFusionTrace) {
	components := make([]alchemyInheritComponent, 0, len(group))
	for _, row := range group {
		components = append(components, row.Component)
	}
	primaryIdx := 0
	if len(components) > 1 {
		primaryIdx = rng.Intn(len(components))
	}
	primary := components[primaryIdx]
	primarySourceRange := primary.ValueRange
	primary.CurrentValue = roundAlchemyValue(mergeInheritValues(components, inheritAdjustedHybridDivisor(medium, derived)), 3)
	primary.ValueRange = primarySourceRange
	clampInheritComponentToSourceCap(&primary, capMultiplier)
	primary.FusionKind = fusionKind
	primary.RetentionReason = fusionKind
	primary.SourceKeys = uniqueComponentSourceKeys(components)
	primary.SourceItems = uniqueComponentItems(components)
	primary.Ancestry = ancestryFromRetained(group)
	primary.Parents = uniqueParentsFromAncestry(primary.Ancestry)
	fillInheritComponentSourceFields(&primary)
	trace := alchemyInheritFusionTrace{
		Attr:          primary.Attr,
		FusionKind:    primary.FusionKind,
		PrimarySource: primary.SourceKey,
		SourceKeys:    primary.SourceKeys,
		Value:         primary.CurrentValue,
	}
	return primary, trace
}

func inheritAttrModsFromComponents(components []alchemyInheritComponent) map[string]float64 {
	attrMods := map[string]float64{}
	for _, component := range components {
		attr := strings.TrimSpace(component.Attr)
		if attr == "" {
			continue
		}
		attrMods[attr] = roundAlchemyValue(attrMods[attr]+component.CurrentValue, 3)
	}
	return attrMods
}

func clampInheritComponentToSourceCap(component *alchemyInheritComponent, capMultiplier float64) {
	if component == nil || strings.TrimSpace(component.Attr) == "" || component.ComponentKind == "effect" {
		return
	}
	minValue, maxValue, ok := inheritComponentSourceCapBounds(*component, capMultiplier)
	if !ok {
		return
	}
	component.CurrentValue = roundAlchemyValue(clampAlchemyFloat(component.CurrentValue, minValue, maxValue), 3)
}

func inheritComponentSourceCapBounds(component alchemyInheritComponent, capMultiplier float64) (float64, float64, bool) {
	if capMultiplier < 1 || math.IsNaN(capMultiplier) || math.IsInf(capMultiplier, 0) {
		capMultiplier = 3
	}
	minValue := component.ValueRange.Min
	maxValue := component.ValueRange.Max
	if math.IsNaN(minValue) || math.IsNaN(maxValue) || math.IsInf(minValue, 0) || math.IsInf(maxValue, 0) {
		return 0, 0, false
	}
	if maxValue < minValue {
		minValue, maxValue = maxValue, minValue
	}
	if minValue == 0 && maxValue == 0 {
		return 0, 0, false
	}
	if minValue < 0 && maxValue > 0 {
		return minValue * capMultiplier, maxValue * capMultiplier, true
	}
	if maxValue <= 0 {
		return minValue * capMultiplier, 0, true
	}
	return 0, maxValue * capMultiplier, true
}

func mergeInheritValues(components []alchemyInheritComponent, divisor float64) float64 {
	if len(components) == 0 {
		return 0
	}
	if divisor < 1 {
		divisor = 1
	}
	sum := 0.0
	for _, component := range components {
		sum += component.CurrentValue
	}
	denom := 1 + float64(len(components)-1)*(divisor-1)
	if denom <= 0 {
		denom = 1
	}
	return roundAlchemyValue(sum/denom, 3)
}

func ancestryFromRetained(group []inheritRetainedGene) []alchemyInheritAncestry {
	out := []alchemyInheritAncestry{}
	for _, row := range group {
		component := row.Component
		if len(component.Ancestry) > 0 {
			for _, ancestry := range component.Ancestry {
				if ancestry.Decision == "" {
					ancestry.Decision = row.RetentionReason
				}
				out = append(out, ancestry)
			}
			continue
		}
		out = append(out, alchemyInheritAncestry{
			Parent:         row.Gene.Parent,
			ComponentIndex: row.Gene.ComponentIndex,
			SourceKey:      component.SourceKey,
			SourceType:     component.SourceType,
			SourceID:       component.SourceID,
			ComponentKind:  component.ComponentKind,
			ItemID:         component.ItemID,
			TraitID:        component.TraitID,
			AffixID:        component.AffixID,
			AffixNameKey:   component.AffixNameKey,
			ComboRecipeID:  component.ComboRecipeID,
			MutationID:     component.MutationID,
			Attr:           component.Attr,
			CurrentValue:   component.CurrentValue,
			Decision:       row.RetentionReason,
		})
	}
	return out
}

func uniqueComponentSourceKeys(components []alchemyInheritComponent) []string {
	values := []string{}
	for _, component := range components {
		if len(component.SourceKeys) > 0 {
			values = append(values, component.SourceKeys...)
		} else {
			values = append(values, component.SourceKey)
		}
	}
	return uniqueStrings(values)
}

func uniqueComponentItems(components []alchemyInheritComponent) []string {
	values := []string{}
	for _, component := range components {
		if len(component.SourceItems) > 0 {
			values = append(values, component.SourceItems...)
		} else {
			values = append(values, component.ItemID)
		}
	}
	return uniqueStrings(values)
}

func uniqueParentsFromAncestry(ancestry []alchemyInheritAncestry) []string {
	values := []string{}
	for _, row := range ancestry {
		values = append(values, row.Parent)
	}
	return uniqueStrings(values)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func inheritAdjustedIgnoreKeepChance(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) float64 {
	if medium == nil {
		return 0
	}
	add := derived.LuckyChance * medium.Modifiers.LuckyScaling.IgnoreKeepChancePerPoint
	if capValue := medium.Modifiers.LuckyScaling.IgnoreKeepChanceCap; capValue > 0 && add > capValue {
		add = capValue
	}
	return clampAlchemyFloat(medium.Modifiers.IgnoreKeepChance+add, 0, 1)
}

func inheritAdjustedIgnoreKeepChanceForComponent(medium *catalog.AlchemyInheritMedium, derived alchemyDerived, component alchemyInheritComponent) float64 {
	chance := inheritAdjustedIgnoreKeepChance(medium, derived)
	if medium == nil {
		return chance
	}
	if strings.TrimSpace(component.Attr) == "" || component.ComponentKind == "effect" {
		base := medium.Modifiers.PositiveIgnoreKeepChanceBase
		if base <= 0 {
			base = 0.4
		}
		return clampAlchemyFloat(base+chance, 0, 1)
	}
	ratio := positiveAlchemyInheritanceRatio(derived.Stability)
	if component.CurrentValue < 0 {
		base := medium.Modifiers.NegativeIgnoreKeepChanceBase
		if base <= 0 {
			base = 0.9
		}
		chance = base*ratio - medium.Modifiers.NegativeIgnoreSuppressChanceBonus
	} else {
		base := medium.Modifiers.PositiveIgnoreKeepChanceBase
		if base <= 0 {
			base = 0.5
		}
		chance = 1 - (1-base)*ratio + medium.Modifiers.PositiveIgnoreKeepChanceBonus
	}
	return clampAlchemyFloat(chance, 0, 1)
}

func positiveAlchemyInheritanceRatio(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	return 1 / math.Max(0.01, 1+value)
}

func effectiveAlchemyInheritInheritSlots(base int, precision float64, scalePerPoint float64) int {
	if base <= 0 {
		return 0
	}
	if precision < 0 || math.IsNaN(precision) || math.IsInf(precision, 0) {
		precision = 0
	}
	if scalePerPoint < 0 || math.IsNaN(scalePerPoint) || math.IsInf(scalePerPoint, 0) {
		scalePerPoint = 0
	}
	return int(math.Floor(float64(base)*(1+precision*scalePerPoint) + 1e-9))
}

func effectiveAlchemyInheritSuppressSlots(base int, stability float64, bonusPerPoint float64) int {
	if base < 0 {
		base = 0
	}
	if stability < 0 || math.IsNaN(stability) || math.IsInf(stability, 0) {
		stability = 0
	}
	if bonusPerPoint < 0 || math.IsNaN(bonusPerPoint) || math.IsInf(bonusPerPoint, 0) {
		bonusPerPoint = 0
	}
	return base + int(math.Floor(stability*bonusPerPoint+1e-9))
}

func (s *Store) effectiveAlchemyInheritInheritSlots(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) int {
	cfg := s.effectiveAlchemyInheritConfig()
	bonus := 0
	if medium != nil && medium.Modifiers.InheritSlots > 0 {
		bonus = medium.Modifiers.InheritSlots
	}
	return effectiveAlchemyInheritInheritSlots(cfg.BaseInheritSlots, derived.Precision, cfg.InheritSlotPrecisionScale) + bonus
}

func (s *Store) effectiveAlchemyInheritSuppressSlots(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) int {
	cfg := s.effectiveAlchemyInheritConfig()
	bonus := 0
	if medium != nil && medium.Modifiers.SuppressSlots > 0 {
		bonus = medium.Modifiers.SuppressSlots
	}
	return effectiveAlchemyInheritSuppressSlots(cfg.BaseSuppressSlots, derived.Stability, cfg.SuppressSlotStabilityBonus) + bonus
}

func (s *Store) effectiveAlchemyInheritRules(medium *catalog.AlchemyInheritMedium) catalog.AlchemyInheritMedium {
	cfg := s.effectiveAlchemyInheritConfig()
	rules := catalog.AlchemyInheritMedium{
		Modifiers: catalog.AlchemyInheritModifiers{
			IgnoreKeepChance:             0,
			PositiveIgnoreKeepChanceBase: cfg.PositiveIgnoreKeepChanceBase,
			NegativeIgnoreKeepChanceBase: cfg.NegativeIgnoreKeepChanceBase,
			HybridMergeDivisor:           cfg.HybridMergeDivisor,
			InbreedMutationChance:        cfg.InbreedMutationChance,
			InbreedGoodChance:            cfg.InbreedGoodChance,
			InbreedGoodMultiplierRange:   cfg.InbreedGoodMultiplierRange,
			InbreedBadMultiplierRange:    cfg.InbreedBadMultiplierRange,
			MutationChance:               cfg.MutationChance,
		},
	}
	if medium == nil {
		return rules
	}
	rules.ID = medium.ID
	rules.ItemID = medium.ItemID
	rules.NameKey = medium.NameKey
	rules.DescKey = medium.DescKey
	rules.ConsumePerCraft = medium.ConsumePerCraft
	rules.MinAlchemyLevel = medium.MinAlchemyLevel
	rules.Active = medium.Active
	rules.Disabled = medium.Disabled
	rules.Modifiers.InheritSlots = medium.Modifiers.InheritSlots
	rules.Modifiers.SuppressSlots = medium.Modifiers.SuppressSlots
	rules.Modifiers.IgnoreKeepChance = clampAlchemyFloat(medium.Modifiers.IgnoreKeepChance, 0, 1)
	rules.Modifiers.PositiveIgnoreKeepChanceBonus = clampAlchemyFloat(medium.Modifiers.PositiveIgnoreKeepChanceBonus, 0, 1)
	rules.Modifiers.NegativeIgnoreSuppressChanceBonus = clampAlchemyFloat(medium.Modifiers.NegativeIgnoreSuppressChanceBonus, 0, 1)
	rules.Modifiers.InheritCost = medium.Modifiers.InheritCost
	rules.Modifiers.SuppressCost = medium.Modifiers.SuppressCost
	rules.Modifiers.PrecisionScaling = medium.Modifiers.PrecisionScaling
	rules.Modifiers.LuckyScaling = medium.Modifiers.LuckyScaling
	rules.Modifiers.StabilityScaling = medium.Modifiers.StabilityScaling
	if medium.Modifiers.HybridMergeDivisor >= 1 {
		rules.Modifiers.HybridMergeDivisor = medium.Modifiers.HybridMergeDivisor
	}
	if medium.Modifiers.InbreedMutationChance >= 0 && medium.Modifiers.InbreedMutationChance <= 1 {
		rules.Modifiers.InbreedMutationChance = medium.Modifiers.InbreedMutationChance
	}
	if medium.Modifiers.InbreedGoodChance >= 0 && medium.Modifiers.InbreedGoodChance <= 1 {
		rules.Modifiers.InbreedGoodChance = medium.Modifiers.InbreedGoodChance
	}
	if medium.Modifiers.InbreedGoodMultiplierRange.Min > 0 && medium.Modifiers.InbreedGoodMultiplierRange.Max >= medium.Modifiers.InbreedGoodMultiplierRange.Min {
		rules.Modifiers.InbreedGoodMultiplierRange = medium.Modifiers.InbreedGoodMultiplierRange
	}
	if medium.Modifiers.InbreedBadMultiplierRange.Min > 0 && medium.Modifiers.InbreedBadMultiplierRange.Max >= medium.Modifiers.InbreedBadMultiplierRange.Min {
		rules.Modifiers.InbreedBadMultiplierRange = medium.Modifiers.InbreedBadMultiplierRange
	}
	if medium.Modifiers.MutationChance >= 0 && medium.Modifiers.MutationChance <= 1 {
		rules.Modifiers.MutationChance = medium.Modifiers.MutationChance
	}
	return rules
}

func (s *Store) effectiveAlchemyInheritConfig() AlchemyInheritRuntimeConfig {
	cfg := s.config.Alchemy.Inherit
	if cfg.BaseInheritSlots <= 0 {
		cfg.BaseInheritSlots = 3
	}
	if cfg.BaseSuppressSlots < 0 {
		cfg.BaseSuppressSlots = 0
	}
	if cfg.InheritSlotPrecisionScale <= 0 || math.IsNaN(cfg.InheritSlotPrecisionScale) || math.IsInf(cfg.InheritSlotPrecisionScale, 0) {
		cfg.InheritSlotPrecisionScale = 1
	}
	if cfg.SuppressSlotStabilityBonus <= 0 || math.IsNaN(cfg.SuppressSlotStabilityBonus) || math.IsInf(cfg.SuppressSlotStabilityBonus, 0) {
		cfg.SuppressSlotStabilityBonus = 1
	}
	if cfg.PositiveIgnoreKeepChanceBase <= 0 || cfg.PositiveIgnoreKeepChanceBase > 1 {
		cfg.PositiveIgnoreKeepChanceBase = 0.4
	}
	if cfg.NegativeIgnoreKeepChanceBase <= 0 || cfg.NegativeIgnoreKeepChanceBase > 1 {
		cfg.NegativeIgnoreKeepChanceBase = 1
	}
	if cfg.HybridMergeDivisor < 1 {
		cfg.HybridMergeDivisor = 1.9
	}
	if cfg.InbreedMutationChance <= 0 || cfg.InbreedMutationChance > 1 {
		cfg.InbreedMutationChance = 0.8
	}
	if cfg.InbreedGoodChance <= 0 || cfg.InbreedGoodChance > 1 {
		cfg.InbreedGoodChance = 0.05
	}
	if cfg.InbreedGoodMultiplierRange.Min <= 0 || cfg.InbreedGoodMultiplierRange.Max < cfg.InbreedGoodMultiplierRange.Min {
		cfg.InbreedGoodMultiplierRange = catalog.AlchemyInheritMultiplierRange{Min: 1.1, Max: 1.35}
	}
	if cfg.InbreedBadMultiplierRange.Min <= 0 || cfg.InbreedBadMultiplierRange.Max < cfg.InbreedBadMultiplierRange.Min {
		cfg.InbreedBadMultiplierRange = catalog.AlchemyInheritMultiplierRange{Min: 0.35, Max: 0.8}
	}
	if cfg.MutationChance <= 0 || cfg.MutationChance > 1 {
		cfg.MutationChance = 0.05
	}
	if cfg.NumericValueCapMultiplier < 1 {
		cfg.NumericValueCapMultiplier = 3
	}
	return cfg
}

func (s *Store) alchemyInheritNumericCapMultiplier() float64 {
	return s.effectiveAlchemyInheritConfig().NumericValueCapMultiplier
}

func inheritAdjustedHybridDivisor(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) float64 {
	if medium == nil {
		return 1
	}
	reduction := derived.Precision * medium.Modifiers.PrecisionScaling.HybridDivisorReductionPerPoint
	if capValue := medium.Modifiers.PrecisionScaling.HybridDivisorReductionCap; capValue > 0 && reduction > capValue {
		reduction = capValue
	}
	divisor := medium.Modifiers.HybridMergeDivisor - reduction
	if divisor < 1 {
		return 1
	}
	return divisor
}

func inheritAdjustedInbreedMutationChance(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) float64 {
	if medium == nil {
		return 0
	}
	reduction := derived.Stability * medium.Modifiers.StabilityScaling.InbreedMutationChanceReductionPerPoint
	if capValue := medium.Modifiers.StabilityScaling.InbreedMutationChanceReductionCap; capValue > 0 && reduction > capValue {
		reduction = capValue
	}
	return clampAlchemyFloat(medium.Modifiers.InbreedMutationChance-reduction, 0, 1)
}

func inheritAdjustedInbreedGoodChance(medium *catalog.AlchemyInheritMedium, derived alchemyDerived) float64 {
	if medium == nil {
		return 0
	}
	add := derived.LuckyChance * medium.Modifiers.LuckyScaling.InbreedGoodChancePerPoint
	if capValue := medium.Modifiers.LuckyScaling.InbreedGoodChanceCap; capValue > 0 && add > capValue {
		add = capValue
	}
	return clampAlchemyFloat(medium.Modifiers.InbreedGoodChance+add, 0, 1)
}

func applyInbreedGood(value float64, medium *catalog.AlchemyInheritMedium, rng *rand.Rand) float64 {
	mult := sampleInheritMultiplier(medium.Modifiers.InbreedGoodMultiplierRange, rng)
	if value < 0 {
		return value / mult
	}
	return value * mult
}

func applyInbreedBad(value float64, medium *catalog.AlchemyInheritMedium, derived alchemyDerived, rng *rand.Rand) float64 {
	rngRange := medium.Modifiers.InbreedBadMultiplierRange
	bonus := derived.Stability * medium.Modifiers.StabilityScaling.BadMultiplierFloorBonusPerPoint
	if capValue := medium.Modifiers.StabilityScaling.BadMultiplierFloorBonusCap; capValue > 0 && bonus > capValue {
		bonus = capValue
	}
	rngRange.Min = math.Min(0.95, rngRange.Min+bonus)
	rngRange.Max = math.Min(0.98, rngRange.Max+bonus)
	if rngRange.Max < rngRange.Min {
		rngRange.Max = rngRange.Min
	}
	mult := sampleInheritMultiplier(rngRange, rng)
	if value < 0 {
		return value / mult
	}
	return value * mult
}

func sampleInheritMultiplier(valueRange catalog.AlchemyInheritMultiplierRange, rng *rand.Rand) float64 {
	minValue := valueRange.Min
	maxValue := valueRange.Max
	if minValue <= 0 {
		minValue = 1
	}
	if maxValue < minValue {
		maxValue = minValue
	}
	if rng == nil || maxValue == minValue {
		return minValue
	}
	return minValue + rng.Float64()*(maxValue-minValue)
}

func (s *Store) extractAlchemyInheritComponentsFromParentSlot(slot EquipmentAffixSlot, parentTag string) []alchemyInheritComponent {
	if strings.HasPrefix(strings.TrimSpace(slot.AffixID), "custom:") {
		return s.extractCustomAlchemyInheritComponentsFromSlot(slot, parentTag)
	}
	return s.projectOrdinaryAffixToInheritComponents(slot, parentTag)
}

func (s *Store) extractCustomAlchemyInheritComponentsFromSlot(slot EquipmentAffixSlot, parentTag string) []alchemyInheritComponent {
	if slot.EffectParams == nil {
		return nil
	}
	raw, ok := slot.EffectParams["alchemyComponents"]
	if !ok || raw == nil {
		return nil
	}
	rows := anySliceFromAlchemyComponents(raw)
	parentSourceItems := sourceItemsFromInheritRows(rows)
	out := make([]alchemyInheritComponent, 0, len(rows))
	for idx, row := range rows {
		component, ok := inheritComponentFromAny(row)
		if !ok {
			continue
		}
		if strings.TrimSpace(component.SourceKind) == "combo_material" {
			continue
		}
		component.SourceKey = normalizeInheritSourceKey(component, idx)
		component.ImmediateParent = parentTag
		if component.ComponentKind == "" {
			if component.Attr == "" {
				component.ComponentKind = "effect"
			} else {
				component.ComponentKind = "attr"
			}
		}
		if component.SourceKind == "" {
			component.SourceKind = "inherit_from_" + parentTag
		}
		if component.SourceKind == "combo_output" && component.ComboRecipeID != "" {
			component.EffectParams = inheritComplexEffectParams(slot.EffectParams)
		}
		if component.ComboRecipeID != "" && component.ItemID == "" && len(component.SourceItems) == 0 && len(parentSourceItems) > 0 {
			component.SourceItems = append([]string{}, parentSourceItems...)
		}
		if component.Attr == "" && len(component.EffectParams) == 0 {
			continue
		}
		fillInheritComponentSourceFields(&component)
		out = append(out, component)
	}
	return filterBundledComboInheritComponents(out)
}

func filterBundledComboInheritComponents(components []alchemyInheritComponent) []alchemyInheritComponent {
	bundledItems := map[string]bool{}
	for _, component := range components {
		if strings.TrimSpace(component.ComboRecipeID) == "" {
			continue
		}
		for _, itemID := range component.SourceItems {
			itemID = strings.TrimSpace(itemID)
			if itemID != "" {
				bundledItems[itemID] = true
			}
		}
	}
	if len(bundledItems) == 0 {
		return components
	}
	out := make([]alchemyInheritComponent, 0, len(components))
	for _, component := range components {
		if strings.TrimSpace(component.ComboRecipeID) != "" {
			out = append(out, component)
			continue
		}
		if component.ItemID != "" && bundledItems[component.ItemID] {
			continue
		}
		out = append(out, component)
	}
	return out
}

func sourceItemsFromInheritRows(rows []any) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(itemID string) {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || seen[itemID] {
			return
		}
		seen[itemID] = true
		out = append(out, itemID)
	}
	for idx, row := range rows {
		component, ok := inheritComponentFromAny(row)
		if !ok {
			continue
		}
		component.SourceKey = normalizeInheritSourceKey(component, idx)
		add(component.ItemID)
		for _, itemID := range component.SourceItems {
			add(itemID)
		}
		for _, ancestry := range component.Ancestry {
			add(ancestry.ItemID)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) projectOrdinaryAffixToInheritComponents(slot EquipmentAffixSlot, parentTag string) []alchemyInheritComponent {
	affixID := strings.TrimSpace(slot.AffixID)
	def := s.gameData.AffixDefinitionByID(affixID)
	if def == nil || !def.Active {
		return nil
	}
	params := slot.EffectParams
	if len(params) == 0 {
		params = resolveAffixEffectParamsByRarity(def.EffectParams, slot.RarityID)
	}
	if params == nil {
		params = map[string]any{}
	}
	attrMods := slot.AttrMods
	if len(attrMods) == 0 {
		attrMods = rollAffixEffects(*def, slot.RarityID, slot.RollValue)
	}
	out := make([]alchemyInheritComponent, 0, len(attrMods)+1)
	attrKeys := make([]string, 0, len(attrMods))
	for attr := range attrMods {
		if strings.TrimSpace(attr) != "" {
			attrKeys = append(attrKeys, attr)
		}
	}
	sort.Strings(attrKeys)
	for _, attr := range attrKeys {
		current := roundAlchemyValue(attrMods[attr], 3)
		valueRange := ordinaryAffixAttrRange(def, params, attr, current)
		component := alchemyInheritComponent{
			SourceKind:               "ordinary_affix",
			SourceKey:                "affix:" + affixID + ":" + attr,
			SourceType:               "affix",
			SourceID:                 affixID,
			ImmediateParent:          parentTag,
			ComponentKind:            "attr",
			AffixID:                  affixID,
			AffixNameKey:             def.NameKey,
			Attr:                     attr,
			ValueRange:               valueRange,
			CurrentValue:             current,
			MaxEffectivePerEquipment: def.MaxEffectivePerEquipment,
			MaxEffectivePerBody:      def.MaxEffectivePerBody,
		}
		out = append(out, component)
	}
	if complex := inheritComplexEffectParams(params); len(complex) > 0 || len(attrKeys) == 0 {
		component := alchemyInheritComponent{
			SourceKind:               "ordinary_affix",
			SourceKey:                "affix:" + affixID + ":effect",
			SourceType:               "affix",
			SourceID:                 affixID,
			ImmediateParent:          parentTag,
			ComponentKind:            "effect",
			AffixID:                  affixID,
			AffixNameKey:             def.NameKey,
			EffectType:               def.EffectType,
			EffectParams:             complex,
			MaxEffectivePerEquipment: def.MaxEffectivePerEquipment,
			MaxEffectivePerBody:      def.MaxEffectivePerBody,
		}
		out = append(out, component)
	}
	return out
}

func ordinaryAffixAttrRange(def *catalog.AffixDefinition, params map[string]any, attr string, fallback float64) catalog.AlchemyValueRange {
	valueRange := catalog.AlchemyValueRange{Min: fallback, Max: fallback}
	rawAttrMods, ok := params["attrMods"]
	if !ok || rawAttrMods == nil {
		return valueRange
	}
	nodes, ok := rawAttrMods.(map[string]any)
	if !ok {
		return valueRange
	}
	rawRange, ok := nodes[attr]
	if !ok {
		return valueRange
	}
	if rangeMap, ok := rawRange.(map[string]any); ok {
		minValue, hasMin := numericFromAny(rangeMap["min"])
		maxValue, hasMax := numericFromAny(rangeMap["max"])
		if hasMin && hasMax {
			return catalog.AlchemyValueRange{Min: roundByPrecision(minValue, def.ValuePrecision), Max: roundByPrecision(maxValue, def.ValuePrecision)}
		}
		if value, ok := numericFromAny(rawRange); ok {
			return catalog.AlchemyValueRange{Min: roundByPrecision(value, def.ValuePrecision), Max: roundByPrecision(value, def.ValuePrecision)}
		}
	}
	if value, ok := numericFromAny(rawRange); ok {
		return catalog.AlchemyValueRange{Min: roundByPrecision(value, def.ValuePrecision), Max: roundByPrecision(value, def.ValuePrecision)}
	}
	return valueRange
}

func inheritComplexEffectParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range params {
		normalized := strings.TrimSpace(key)
		if normalized == "" || normalized == "attrMods" || normalized == "attr" || normalized == "alchemyComponents" || normalized == "alchemyInheritSummary" {
			continue
		}
		if normalized == "alchemyKind" || normalized == "inheritSourceKind" || normalized == "inheritFamily" || normalized == "inheritOrphan" || normalized == "inheritOrphanReason" {
			continue
		}
		out[normalized] = cloneAlchemyAny(value)
	}
	return out
}

func mergeInheritedEffectParams(target map[string]any, source map[string]any) {
	if len(target) == 0 && len(source) == 0 {
		return
	}
	for key, value := range source {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if key == "battleHooks" || key == "battle_hooks" || key == "hooks" {
			appendUniqueEffectParamArray(target, "battleHooks", value)
			continue
		}
		if key == "effectMutations" || key == "effect_mutations" {
			appendUniqueEffectParamArray(target, "effectMutations", value)
			continue
		}
		if _, exists := target[key]; !exists {
			target[key] = cloneAlchemyAny(value)
		}
	}
}

func appendUniqueEffectParamArray(target map[string]any, key string, raw any) {
	rows := anySliceFromAlchemyComponents(raw)
	if len(rows) == 0 {
		return
	}
	existing := anySliceFromAlchemyComponents(target[key])
	seen := map[string]bool{}
	out := make([]any, 0, len(existing)+len(rows))
	for _, row := range existing {
		token := stableAlchemyJSON(row)
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, cloneAlchemyAny(row))
	}
	for _, row := range rows {
		token := stableAlchemyJSON(row)
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, cloneAlchemyAny(row))
	}
	if len(out) > 0 {
		target[key] = out
	}
}

func stableAlchemyJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

func cloneAlchemyAny(value any) any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return value
	}
	return out
}

func anySliceFromAlchemyComponents(raw any) []any {
	switch typed := raw.(type) {
	case []any:
		return typed
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, row := range typed {
			out = append(out, row)
		}
		return out
	case []alchemyCraftComponent:
		out := make([]any, 0, len(typed))
		for _, row := range typed {
			out = append(out, row)
		}
		return out
	case []alchemyInheritComponent:
		out := make([]any, 0, len(typed))
		for _, row := range typed {
			out = append(out, row)
		}
		return out
	default:
		return nil
	}
}

func inheritComponentFromAny(row any) (alchemyInheritComponent, bool) {
	switch typed := row.(type) {
	case alchemyInheritComponent:
		typed.Attr = cleanAlchemyComponentAttr(typed.Attr)
		if typed.Attr == "" && typed.ComboRecipeID != "" {
			typed.ComponentKind = "effect"
		}
		return typed, true
	case alchemyCraftComponent:
		attr := cleanAlchemyComponentAttr(typed.Attr)
		componentKind := "attr"
		if attr == "" && typed.ComboRecipeID != "" {
			componentKind = "effect"
		}
		return alchemyInheritComponent{
			SourceKind:               typed.SourceKind,
			ComponentKind:            componentKind,
			ItemID:                   typed.ItemID,
			TraitID:                  typed.TraitID,
			ComboRecipeID:            typed.ComboRecipeID,
			Family:                   typed.Family,
			Attr:                     attr,
			Quantity:                 typed.Quantity,
			ValueRange:               typed.ValueRange,
			CurrentValue:             typed.CurrentValue,
			MaxEffectivePerEquipment: typed.MaxEffectivePerEquipment,
			MaxEffectivePerBody:      typed.MaxEffectivePerBody,
			OperationEffects:         typed.OperationEffects,
		}, true
	case map[string]any:
		component := alchemyInheritComponent{
			SourceKind:               stringFromAny(typed["sourceKind"]),
			SourceKey:                stringFromAny(typed["sourceKey"]),
			SourceType:               stringFromAny(typed["sourceType"]),
			SourceID:                 stringFromAny(typed["sourceId"]),
			ImmediateParent:          stringFromAny(typed["immediateParent"]),
			ComponentKind:            stringFromAny(typed["componentKind"]),
			ItemID:                   stringFromAny(typed["itemId"]),
			TraitID:                  stringFromAny(typed["traitId"]),
			AffixID:                  stringFromAny(typed["affixId"]),
			AffixNameKey:             stringFromAny(typed["affixNameKey"]),
			ComboRecipeID:            stringFromAny(typed["comboRecipeId"]),
			MutationID:               stringFromAny(typed["mutationId"]),
			Family:                   stringFromAny(typed["family"]),
			Attr:                     cleanAlchemyComponentAttr(stringFromAny(typed["attr"])),
			EffectType:               stringFromAny(typed["effectType"]),
			Quantity:                 int64(numberFromAny(typed["quantity"])),
			CurrentValue:             numberFromAny(typed["currentValue"]),
			MaxEffectivePerEquipment: int64(numberFromAny(typed["maxEffectivePerEquipment"])),
			MaxEffectivePerBody:      int64(numberFromAny(typed["maxEffectivePerBody"])),
			FusionKind:               stringFromAny(typed["fusionKind"]),
			RetentionReason:          stringFromAny(typed["retentionReason"]),
		}
		if rangeMap, ok := typed["valueRange"].(map[string]any); ok {
			component.ValueRange = catalog.AlchemyValueRange{Min: numberFromAny(rangeMap["min"]), Max: numberFromAny(rangeMap["max"])}
		}
		if stamp, err := decodeAlchemyOperationEffectStamp(typed["operationEffects"]); err == nil {
			component.OperationEffects = stamp
		}
		component.SourceKeys = stringSliceFromAny(typed["sourceKeys"])
		component.SourceItems = stringSliceFromAny(typed["sourceItems"])
		component.Parents = stringSliceFromAny(typed["parents"])
		if params, ok := typed["effectParams"].(map[string]any); ok {
			component.EffectParams = cloneMap(params)
		}
		if component.Attr == "" && component.ComboRecipeID != "" {
			component.ComponentKind = "effect"
		}
		if component.ComponentKind == "" {
			if component.Attr == "" {
				component.ComponentKind = "effect"
			} else {
				component.ComponentKind = "attr"
			}
		}
		return component, component.Attr != "" || component.ComponentKind == "effect"
	default:
		return alchemyInheritComponent{}, false
	}
}

func stringSliceFromAny(raw any) []string {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func normalizeInheritSourceKey(component alchemyInheritComponent, idx int) string {
	if component.SourceKey != "" {
		return cleanAlchemyComponentSourceKey(component.SourceKey)
	}
	attr := cleanAlchemyComponentAttr(component.Attr)
	if component.AffixID != "" {
		if attr == "" {
			attr = "effect"
		}
		return "affix:" + component.AffixID + ":" + attr
	}
	if component.ItemID != "" {
		traitID := component.TraitID
		if traitID == "" {
			traitID = component.ItemID
		}
		return "trait:" + component.ItemID + ":" + traitID + ":" + attr
	}
	if component.ComboRecipeID != "" {
		if attr == "" {
			attr = "effect"
		}
		return "combo:" + component.ComboRecipeID + ":" + attr
	}
	if component.MutationID != "" {
		return "mutation:" + component.MutationID + ":" + attr
	}
	sourceKind := strings.TrimSpace(component.SourceKind)
	if sourceKind == "" {
		sourceKind = "component"
	}
	return fmt.Sprintf("%s:%d:%s", sourceKind, idx, attr)
}

func cleanAlchemyComponentAttr(attr string) string {
	attr = strings.TrimSpace(attr)
	if attr == "<nil>" || strings.EqualFold(attr, "nil") || strings.EqualFold(attr, "null") || attr == "<null>" {
		return ""
	}
	return attr
}

func cleanAlchemyComponentSourceKey(sourceKey string) string {
	sourceKey = strings.TrimSpace(sourceKey)
	if strings.HasSuffix(sourceKey, ":<nil>") || strings.HasSuffix(sourceKey, ":nil") || strings.HasSuffix(sourceKey, ":<null>") || strings.HasSuffix(sourceKey, ":null") {
		parts := strings.Split(sourceKey, ":")
		parts[len(parts)-1] = "effect"
		return strings.Join(parts, ":")
	}
	return sourceKey
}

func fillInheritComponentSourceFields(component *alchemyInheritComponent) {
	if component.SourceType != "" && component.SourceID != "" {
		return
	}
	if component.AffixID != "" {
		component.SourceType = "affix"
		component.SourceID = component.AffixID
		return
	}
	if component.ItemID != "" {
		component.SourceType = "trait"
		component.SourceID = component.ItemID
		if component.TraitID != "" {
			component.SourceID += ":" + component.TraitID
		}
		return
	}
	if component.ComboRecipeID != "" {
		component.SourceType = "combo"
		component.SourceID = component.ComboRecipeID
		return
	}
	if component.MutationID != "" {
		component.SourceType = "mutation"
		component.SourceID = component.MutationID
		return
	}
	parts := strings.Split(component.SourceKey, ":")
	if len(parts) > 1 {
		component.SourceType = parts[0]
		component.SourceID = strings.Join(parts[1:len(parts)-1], ":")
	}
}

func sampleInheritComponentValue(component alchemyInheritComponent, medium *catalog.AlchemyInheritMedium, derived alchemyDerived, rng *rand.Rand, capMultiplier float64) float64 {
	minValue := component.ValueRange.Min
	maxValue := component.ValueRange.Max
	minValue, maxValue = applyAlchemyDerivedRange(minValue, maxValue, derived.Precision, derived.Stability)
	value := sampleLuckSkewed(minValue, maxValue, derived.LuckyChance, rng)
	component.CurrentValue = roundAlchemyValue(value, 3)
	clampInheritComponentToSourceCap(&component, capMultiplier)
	return component.CurrentValue
}

func (s *Store) applyInheritMutation(components *[]alchemyInheritComponent, attrMods map[string]float64, kind string, medium *catalog.AlchemyInheritMedium, derived alchemyDerived, rng *rand.Rand) (bool, string) {
	if len(*components) == 0 {
		return false, ""
	}
	switch rng.Intn(3) {
	case 0:
		component, ok := s.randomLibraryMutationComponent(kind, medium, derived, rng)
		if !ok {
			return false, ""
		}
		*components = append(*components, component)
		attrMods[component.Attr] = roundAlchemyValue(attrMods[component.Attr]+component.CurrentValue, 3)
		return true, "library"
	case 1:
		attrIndexes := make([]int, 0, len(*components))
		for idx, component := range *components {
			if strings.TrimSpace(component.Attr) != "" {
				attrIndexes = append(attrIndexes, idx)
			}
		}
		if len(attrIndexes) == 0 {
			return false, ""
		}
		idx := attrIndexes[rng.Intn(len(attrIndexes))]
		component := (*components)[idx]
		before := component.CurrentValue
		mult := 1.3 + rng.Float64()*1.2
		component.CurrentValue = roundAlchemyValue(component.CurrentValue*mult, 3)
		clampInheritComponentToSourceCap(&component, s.alchemyInheritNumericCapMultiplier())
		component.FusionKind = strings.Trim(strings.Join([]string{component.FusionKind, "mutation_amplify"}, "+"), "+")
		(*components)[idx] = component
		attrMods[component.Attr] = roundAlchemyValue(attrMods[component.Attr]-before+component.CurrentValue, 3)
		return true, "amplify"
	default:
		attr := s.randomAlchemyMutationAttr(kind, rng)
		if attr == "" {
			return false, ""
		}
		value := roundAlchemyValue(-0.05+rng.Float64()*0.10, 3)
		component := alchemyInheritComponent{
			SourceKind:      "mutation",
			SourceKey:       "mutation:wild:" + attr,
			SourceType:      "mutation",
			SourceID:        "wild",
			MutationID:      "wild",
			Attr:            attr,
			ValueRange:      catalog.AlchemyValueRange{Min: -0.05, Max: 0.05},
			CurrentValue:    value,
			FusionKind:      "mutation_wild",
			RetentionReason: "mutation",
			SourceKeys:      []string{"mutation:wild:" + attr},
		}
		clampInheritComponentToSourceCap(&component, s.alchemyInheritNumericCapMultiplier())
		*components = append(*components, component)
		attrMods[attr] = roundAlchemyValue(attrMods[attr]+component.CurrentValue, 3)
		return true, "wild"
	}
}

func (s *Store) randomLibraryMutationComponent(kind string, medium *catalog.AlchemyInheritMedium, derived alchemyDerived, rng *rand.Rand) (alchemyInheritComponent, bool) {
	candidates := []alchemyInheritComponent{}
	for _, row := range s.gameData.ItemTraits {
		for _, trait := range row.Traits {
			if strings.TrimSpace(trait.Kind) != kind {
				continue
			}
			qty := alchemyTraitUnitBasis(trait)
			for _, effect := range trait.Effects {
				attr := strings.TrimSpace(effect.Attr)
				if attr == "" {
					continue
				}
				component := alchemyInheritComponent{
					SourceKind:               "mutation_library",
					ItemID:                   row.ItemID,
					TraitID:                  trait.TraitID,
					Family:                   trait.Family,
					Attr:                     attr,
					Quantity:                 qty,
					ValueRange:               catalog.AlchemyValueRange{Min: effect.ValuePerUnit.Min * float64(qty), Max: effect.ValuePerUnit.Max * float64(qty)},
					MaxEffectivePerEquipment: trait.MaxEffectivePerEquipment,
					MaxEffectivePerBody:      trait.MaxEffectivePerBody,
					FusionKind:               "mutation_library",
					RetentionReason:          "mutation",
				}
				component.SourceKey = normalizeInheritSourceKey(component, 0)
				fillInheritComponentSourceFields(&component)
				component.SourceKeys = []string{component.SourceKey}
				component.SourceItems = uniqueStrings([]string{component.ItemID})
				candidates = append(candidates, component)
			}
		}
	}
	if len(candidates) == 0 {
		return alchemyInheritComponent{}, false
	}
	component := candidates[rng.Intn(len(candidates))]
	component.CurrentValue = sampleInheritComponentValue(component, medium, derived, rng, s.alchemyInheritNumericCapMultiplier())
	return component, true
}

func (s *Store) randomAlchemyMutationAttr(kind string, rng *rand.Rand) string {
	wantCategory := "battle"
	if kind == "life" || kind == "work" {
		wantCategory = "work"
	}
	candidates := []string{}
	for _, attr := range s.gameData.Attributes {
		if strings.TrimSpace(attr.Category) == wantCategory && strings.TrimSpace(attr.ID) != "" {
			candidates = append(candidates, strings.TrimSpace(attr.ID))
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	return candidates[rng.Intn(len(candidates))]
}

func isMoreFavorableAlchemyValue(candidate, existing float64) bool {
	if candidate >= 0 && existing < 0 {
		return true
	}
	if candidate < 0 && existing >= 0 {
		return false
	}
	if candidate >= 0 {
		return candidate > existing
	}
	return math.Abs(candidate) < math.Abs(existing)
}

func isLessFavorableAlchemyValue(candidate, existing float64) bool {
	if candidate >= 0 && existing < 0 {
		return false
	}
	if candidate < 0 && existing >= 0 {
		return true
	}
	if candidate >= 0 {
		return candidate < existing
	}
	return math.Abs(candidate) > math.Abs(existing)
}

func stableComponentTieLess(left, right alchemyInheritComponent) bool {
	return strings.Join([]string{left.SourceKey, left.ItemID, left.TraitID, left.ComboRecipeID, left.Attr}, "\x00") <
		strings.Join([]string{right.SourceKey, right.ItemID, right.TraitID, right.ComboRecipeID, right.Attr}, "\x00")
}

func majorityFamily(counts map[string]int) string {
	best := ""
	bestCount := -1
	for family, count := range counts {
		if family == "" {
			continue
		}
		if count > bestCount || count == bestCount && family < best {
			best = family
			bestCount = count
		}
	}
	return best
}

func sortedStringSet(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) inheritOutputRarity(fatherRarity, motherRarity string, medium *catalog.AlchemyInheritMedium, suppressUsed bool) string {
	rarityID := fatherRarity
	if s.rarityOrder(motherRarity) > s.rarityOrder(rarityID) {
		rarityID = motherRarity
	}
	if strings.TrimSpace(rarityID) == "" {
		rarityID = "common"
	}
	return rarityID
}

func (s *Store) rarityOneTierDown(rarityID string) string {
	currentOrder := s.rarityOrder(rarityID)
	best := "common"
	bestOrder := int64(-1)
	for _, row := range s.gameData.RarityDefinitions {
		if row.Order < currentOrder && row.Order > bestOrder {
			best = row.ID
			bestOrder = row.Order
		}
	}
	return best
}
