package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type specializationTestExecTx struct {
	statements []string
	arguments  [][]any
	row        pgx.Row
}

type specializationLedgerReplayRow struct {
	operation string
	nodeID    string
	result    []byte
	err       error
}

type specializationBoolRow bool

func (row specializationBoolRow) Scan(destinations ...any) error {
	if len(destinations) != 1 {
		return errors.New("unexpected bool row scan shape")
	}
	destination, ok := destinations[0].(*bool)
	if !ok {
		return errors.New("bool row destination is not *bool")
	}
	*destination = bool(row)
	return nil
}

type specializationEquipmentRows struct {
	rows   [][]any
	index  int
	closed bool
}

func (rows *specializationEquipmentRows) Close() { rows.closed = true }
func (rows *specializationEquipmentRows) Err() error {
	return nil
}
func (rows *specializationEquipmentRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (rows *specializationEquipmentRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (rows *specializationEquipmentRows) Next() bool {
	if rows.index >= len(rows.rows) {
		rows.closed = true
		return false
	}
	rows.index++
	return true
}
func (rows *specializationEquipmentRows) Scan(destinations ...any) error {
	if rows.index <= 0 || rows.index > len(rows.rows) || len(destinations) != len(rows.rows[rows.index-1]) {
		return errors.New("unexpected equipment row scan shape")
	}
	for index, value := range rows.rows[rows.index-1] {
		switch destination := destinations[index].(type) {
		case *int64:
			*destination = value.(int64)
		case *string:
			*destination = value.(string)
		default:
			return errors.New("unexpected equipment row destination")
		}
	}
	return nil
}
func (rows *specializationEquipmentRows) Values() ([]any, error) {
	if rows.index <= 0 || rows.index > len(rows.rows) {
		return nil, errors.New("equipment row is not positioned")
	}
	return append([]any(nil), rows.rows[rows.index-1]...), nil
}
func (rows *specializationEquipmentRows) RawValues() [][]byte { return nil }
func (rows *specializationEquipmentRows) Conn() *pgx.Conn     { return nil }

type specializationEquipmentTx struct {
	rows       pgx.Rows
	row        pgx.Row
	statements []string
	arguments  [][]any
}

func (tx *specializationEquipmentTx) Begin(context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *specializationEquipmentTx) Commit(context.Context) error          { return nil }
func (tx *specializationEquipmentTx) Rollback(context.Context) error        { return nil }
func (tx *specializationEquipmentTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *specializationEquipmentTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}
func (tx *specializationEquipmentTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (tx *specializationEquipmentTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *specializationEquipmentTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.statements = append(tx.statements, sql)
	tx.arguments = append(tx.arguments, append([]any(nil), arguments...))
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func (tx *specializationEquipmentTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return tx.rows, nil
}
func (tx *specializationEquipmentTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return tx.row
}
func (tx *specializationEquipmentTx) Conn() *pgx.Conn { return nil }

func (row specializationLedgerReplayRow) Scan(destinations ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(destinations) != 3 {
		return errors.New("unexpected specialization ledger scan shape")
	}
	operation, ok := destinations[0].(*string)
	if !ok {
		return errors.New("specialization ledger operation destination is not *string")
	}
	nodeID, ok := destinations[1].(*string)
	if !ok {
		return errors.New("specialization ledger node destination is not *string")
	}
	result, ok := destinations[2].(*[]byte)
	if !ok {
		return errors.New("specialization ledger result destination is not *[]byte")
	}
	*operation = row.operation
	*nodeID = row.nodeID
	*result = append((*result)[:0], row.result...)
	return nil
}

func (tx *specializationTestExecTx) Begin(context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *specializationTestExecTx) Commit(context.Context) error          { return nil }
func (tx *specializationTestExecTx) Rollback(context.Context) error        { return nil }
func (tx *specializationTestExecTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *specializationTestExecTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}
func (tx *specializationTestExecTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (tx *specializationTestExecTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *specializationTestExecTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.statements = append(tx.statements, sql)
	tx.arguments = append(tx.arguments, append([]any(nil), arguments...))
	if strings.Contains(sql, "UPDATE player_specialization_allocations") {
		if len(arguments) < 3 {
			return pgconn.CommandTag{}, errors.New("missing repriced allocation ids")
		}
		nodeIDs, ok := arguments[2].([]string)
		if !ok {
			return pgconn.CommandTag{}, errors.New("repriced allocation ids are not []string")
		}
		return pgconn.NewCommandTag(fmt.Sprintf("UPDATE %d", len(nodeIDs))), nil
	}
	if strings.Contains(sql, "UPDATE player_specialization_states") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return pgconn.NewCommandTag("DELETE 2"), nil
}
func (tx *specializationTestExecTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}
func (tx *specializationTestExecTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return tx.row
}
func (tx *specializationTestExecTx) Conn() *pgx.Conn { return nil }

func loadRealSpecializationTreeForTest(t *testing.T) specializationTreeSpec {
	t.Helper()
	raw, err := os.ReadFile("../../../packages/game-data/source/json/specialization_trees.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Trees []catalog.SpecializationTree `json:"specialization_trees"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range source.Trees {
		if candidate.ID != playerCombatSpecializationTreeID {
			continue
		}
		tree, err := specializationTreeFromRaw(candidate)
		if err != nil {
			t.Fatal(err)
		}
		return tree
	}
	t.Fatal("authoritative player specialization tree missing")
	return specializationTreeSpec{}
}

func TestSpecializationV6CanonicalTopologyHash(t *testing.T) {
	tree := loadRealSpecializationTreeForTest(t)
	type levelContract struct {
		Level             int                       `json:"level"`
		Cost              int                       `json:"cost"`
		EquipmentTopology []specializationGrantSpec `json:"equipmentTopology"`
	}
	type nodeContract struct {
		ID               string                         `json:"id"`
		Kind             string                         `json:"kind"`
		SchoolIDs        []string                       `json:"schoolIds"`
		ExclusiveGroupID string                         `json:"exclusiveGroupId"`
		Requirements     specializationRequirementsSpec `json:"requirements"`
		Levels           []levelContract                `json:"levels"`
	}
	type schoolContract struct {
		ID           string `json:"id"`
		AttributeKey string `json:"attributeKey"`
	}
	topology := struct {
		ID                 string                        `json:"id"`
		Disabled           bool                          `json:"disabled"`
		SchemaVersion      int                           `json:"schemaVersion"`
		RulesetVersion     string                        `json:"rulesetVersion"`
		OwnerType          string                        `json:"ownerType"`
		InitialPoints      int                           `json:"initialPoints"`
		MaxPoints          int                           `json:"maxPoints"`
		MaxActiveRootNodes int                           `json:"maxActiveRootNodes"`
		Unlock             specializationUnlockSpec      `json:"unlock"`
		PointBands         []specializationPointBandSpec `json:"pointBands"`
		Schools            []schoolContract              `json:"schools"`
		Nodes              []nodeContract                `json:"nodes"`
	}{
		ID: tree.ID, Disabled: tree.Disabled, SchemaVersion: tree.SchemaVersion,
		RulesetVersion: tree.RulesetVersion, OwnerType: tree.OwnerType,
		InitialPoints: tree.InitialPoints, MaxPoints: tree.MaxPoints,
		MaxActiveRootNodes: tree.MaxActiveRootNodes,
		Unlock:             tree.Unlock, PointBands: tree.PointBands,
	}
	for _, school := range tree.Schools {
		topology.Schools = append(topology.Schools, schoolContract{ID: school.ID, AttributeKey: school.AttributeKey})
	}
	for _, node := range tree.Nodes {
		contract := nodeContract{
			ID: node.ID, Kind: node.Kind, SchoolIDs: node.SchoolIDs,
			ExclusiveGroupID: node.ExclusiveGroupID, Requirements: node.Requirements,
		}
		for _, level := range node.Levels {
			levelRow := levelContract{Level: level.Level, Cost: level.Cost}
			for _, grant := range level.Grants {
				if grant.Kind == "equipment_slot_capacity" {
					levelRow.EquipmentTopology = append(levelRow.EquipmentTopology, grant)
				}
			}
			contract.Levels = append(contract.Levels, levelRow)
		}
		topology.Nodes = append(topology.Nodes, contract)
	}
	payload, err := json.Marshal(topology)
	if err != nil {
		t.Fatal(err)
	}
	const want = "e46e28c860918bcc9fc5856a564d232d474d2f5f3622dd3c7bf544b76b83f487"
	if got := fmt.Sprintf("%x", sha256.Sum256(payload)); got != want {
		t.Fatalf("specialization-v6 canonical topology hash=%s, want %s; review the topology diff, keep v6 for compatible additions, and bump the ruleset only when existing allocations become incompatible", got, want)
	}
}

func TestSpecializationResetIdempotencyReplaysBeforeCostOrEquipmentMutation(t *testing.T) {
	committed := SpecializationMutationResult{
		Committed: true,
		Status: SpecializationStatus{
			TreeID:         playerCombatSpecializationTreeID,
			RulesetVersion: specializationRulesetV6,
			Revision:       8,
		},
		SlotCapacities: map[string]int{"right_hand": 1, "head": 1, "accessory": 4},
	}
	payload, err := json.Marshal(committed)
	if err != nil {
		t.Fatal(err)
	}
	tx := &specializationTestExecTx{row: specializationLedgerReplayRow{operation: "reset", result: payload}}
	result, replayed, err := replaySpecializationMutationTx(
		context.Background(), tx, 42, playerCombatSpecializationTreeID, specializationRulesetV6,
		"reset-once", "reset", "",
	)
	if err != nil || !replayed || !result.Committed || result.Status.Revision != 8 || result.SlotCapacities["right_hand"] != 1 {
		t.Fatalf("reset replay result=%#v replayed=%v err=%v", result, replayed, err)
	}
	if len(tx.statements) != 0 {
		t.Fatalf("idempotent reset replay executed a mutation: %#v", tx.statements)
	}
	raw, err := os.ReadFile("store_specializations.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) ResetPlayerSpecialization")
	if start < 0 {
		t.Fatal("ResetPlayerSpecialization not found")
	}
	body = body[start:]
	replayAt := strings.Index(body, "replaySpecializationMutationTx")
	consumeAt := strings.Index(body, "ConsumeByTemplateTx")
	deleteAllocationsAt := strings.Index(body, "DELETE FROM player_specialization_allocations")
	applyImpactAt := strings.Index(body, "applySpecializationEquipmentImpactTx")
	if replayAt < 0 || consumeAt < 0 || applyImpactAt < 0 || replayAt > consumeAt || replayAt > applyImpactAt {
		t.Fatalf("reset must replay idempotent result before charging or changing equipment")
	}
	recomputeAt := strings.Index(body, "onPlayerAttributesChangedTx")
	reconcileEquipmentAt := strings.Index(body, "reconcileUnmetEquipmentStatusReqTx")
	disableSkillsAt := strings.Index(body, "disableSpecializationSkillsTx")
	projectSlotsAt := strings.Index(body, "result.EffectiveSlots = afterProfile.Slots")
	writeLedgerAt := -1
	if projectSlotsAt >= 0 {
		if relative := strings.Index(body[projectSlotsAt:], "writeSpecializationLedgerTx"); relative >= 0 {
			writeLedgerAt = projectSlotsAt + relative
		}
	}
	if deleteAllocationsAt < consumeAt || applyImpactAt < deleteAllocationsAt || recomputeAt < applyImpactAt || reconcileEquipmentAt < recomputeAt || disableSkillsAt < reconcileEquipmentAt || projectSlotsAt < disableSkillsAt || writeLedgerAt < projectSlotsAt {
		t.Fatalf("reset transaction no longer charges, clears allocations, unequips, recomputes, disables skills, projects slots, and records idempotency in the required order")
	}
}

func TestSpecializationTotalPointsUsesInclusiveBandsAndCap(t *testing.T) {
	tree := specializationTreeSpec{
		InitialPoints: 2,
		MaxPoints:     24,
		Unlock:        specializationUnlockSpec{BattleLevel: 10},
		PointBands: []specializationPointBandSpec{
			{FromLevel: 15, ToLevel: 90, EveryLevels: 5, PointsPerGrant: 1},
			{FromLevel: 120, ToLevel: 420, EveryLevels: 60, PointsPerGrant: 1},
		},
	}
	tests := []struct {
		level int64
		want  int
	}{
		{9, 0}, {10, 2}, {14, 2}, {15, 3}, {90, 18}, {119, 18}, {120, 19}, {420, 24}, {999, 24},
	}
	for _, test := range tests {
		if got := specializationTotalPoints(tree, test.level); got != test.want {
			t.Fatalf("level %d points=%d, want %d", test.level, got, test.want)
		}
	}
}

func TestSpecializationPointBreakdownExplainsLevel120NineteenPoints(t *testing.T) {
	tree := specializationTreeSpec{
		InitialPoints: 2,
		MaxPoints:     24,
		Unlock:        specializationUnlockSpec{BattleLevel: 10},
		PointBands: []specializationPointBandSpec{
			{ID: "early_growth", FromLevel: 15, ToLevel: 90, EveryLevels: 5, PointsPerGrant: 1},
			{ID: "late_growth", FromLevel: 120, ToLevel: 420, EveryLevels: 60, PointsPerGrant: 1},
		},
	}
	breakdown := specializationPointBreakdown(tree, 120, 0)
	if breakdown.UnlockGrantPoints != 2 || breakdown.LevelMilestonePoints != 17 || breakdown.RewardPoints != 0 {
		t.Fatalf("level 120 source totals do not explain 19 points: %#v", breakdown)
	}
	if breakdown.AtCap || breakdown.NextPointBattleLevel == nil || *breakdown.NextPointBattleLevel != 180 || breakdown.NextPointPoints != 1 {
		t.Fatalf("unexpected level 120 next point projection: %#v", breakdown)
	}
	earnedSources := 0
	countedPoints := 0
	for _, source := range breakdown.Sources {
		if source.Earned {
			earnedSources++
		}
		countedPoints += source.CountedPoints
	}
	if earnedSources != 18 || countedPoints != 19 {
		t.Fatalf("unexpected earned sources=%d countedPoints=%d breakdown=%#v", earnedSources, countedPoints, breakdown)
	}
	if got := specializationTotalPoints(tree, 120); got != countedPoints {
		t.Fatalf("breakdown counted points=%d disagree with formula points=%d", countedPoints, got)
	}
	if len(breakdown.MilestoneBands) != 2 {
		t.Fatalf("unexpected milestone band count: %#v", breakdown.MilestoneBands)
	}
	early, late := breakdown.MilestoneBands[0], breakdown.MilestoneBands[1]
	if early.EarnedGrantCount != 16 || early.EarnedPoints != 16 || early.FirstEarnedBattleLevel == nil || *early.FirstEarnedBattleLevel != 15 || early.LastEarnedBattleLevel == nil || *early.LastEarnedBattleLevel != 90 || early.NextMilestoneBattleLevel != nil {
		t.Fatalf("early milestone band does not explain the earned 15-90 points: %#v", early)
	}
	if late.EarnedGrantCount != 1 || late.EarnedPoints != 1 || late.FirstEarnedBattleLevel == nil || *late.FirstEarnedBattleLevel != 120 || late.LastEarnedBattleLevel == nil || *late.LastEarnedBattleLevel != 120 || late.NextMilestoneBattleLevel == nil || *late.NextMilestoneBattleLevel != 180 {
		t.Fatalf("late milestone band does not explain the earned point and next level: %#v", late)
	}
}

func TestSpecializationPointBreakdownProjectsUnlockAndCapWithoutClientMath(t *testing.T) {
	tree := specializationTreeSpec{
		InitialPoints: 2,
		MaxPoints:     24,
		Unlock:        specializationUnlockSpec{BattleLevel: 10},
		PointBands: []specializationPointBandSpec{
			{ID: "growth", FromLevel: 15, ToLevel: 120, EveryLevels: 5, PointsPerGrant: 1},
		},
	}
	locked := specializationPointBreakdown(tree, 9, 0)
	if locked.UnlockGrantPoints != 0 || locked.NextPointBattleLevel == nil || *locked.NextPointBattleLevel != 10 || locked.NextPointPoints != 2 {
		t.Fatalf("locked tree must project its unlock grant as the next source: %#v", locked)
	}
	nearCap := specializationPointBreakdown(tree, 9, 23)
	if nearCap.AtCap || nearCap.NextPointBattleLevel == nil || *nearCap.NextPointBattleLevel != 10 || nearCap.NextPointPoints != 1 {
		t.Fatalf("next point must report its net gain after bonus points and max cap: %#v", nearCap)
	}

	cappedTree := specializationTreeSpec{
		InitialPoints: 20,
		MaxPoints:     24,
		Unlock:        specializationUnlockSpec{BattleLevel: 1},
	}
	capped := specializationPointBreakdown(cappedTree, 1, 10)
	if !capped.AtCap || capped.NextPointBattleLevel != nil || capped.CountedRewardPoints != 4 || capped.CappedPoints != 6 {
		t.Fatalf("bonus points must be explained against the authoritative cap: %#v", capped)
	}
	if len(capped.Sources) != 2 || capped.Sources[1].Kind != specializationPointSourceReward || capped.Sources[1].RequiredBattleLevel != nil {
		t.Fatalf("reward source must remain stable and level-independent: %#v", capped.Sources)
	}
}

func TestSpecializationRealV6PointBreakdownFrontLoadsSixtyAndReachesSixtyFive(t *testing.T) {
	raw, err := os.ReadFile("../../../packages/game-data/source/json/specialization_trees.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Trees []catalog.SpecializationTree `json:"specialization_trees"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	var tree specializationTreeSpec
	for _, candidate := range source.Trees {
		if candidate.ID == playerCombatSpecializationTreeID {
			tree, err = specializationTreeFromRaw(candidate)
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if tree.RulesetVersion != specializationRulesetV6 || tree.MaxPoints != 65 || tree.MaxActiveRootNodes != 3 {
		t.Fatalf("unexpected authoritative point contract: ruleset=%q max=%d", tree.RulesetVersion, tree.MaxPoints)
	}
	if specializationTotalPoints(tree, 200) != 40 || specializationTotalPoints(tree, 300) != 60 {
		t.Fatalf("front-loaded point contract drifted: level200=%d level300=%d", specializationTotalPoints(tree, 200), specializationTotalPoints(tree, 300))
	}
	beforeCap := specializationPointBreakdown(tree, 499, 0)
	if specializationTotalPoints(tree, 499) != 63 || beforeCap.AtCap || beforeCap.NextPointBattleLevel == nil || *beforeCap.NextPointBattleLevel != 500 || beforeCap.NextPointPoints != 2 {
		t.Fatalf("level 499 must project the two-point cap grant: %#v", beforeCap)
	}
	atCap := specializationPointBreakdown(tree, 500, 0)
	if specializationTotalPoints(tree, 500) != 65 || !atCap.AtCap || atCap.UnlockGrantPoints != 2 || atCap.LevelMilestonePoints != 63 || atCap.NextPointBattleLevel != nil || atCap.CappedPoints != 0 {
		t.Fatalf("level 500 must explain all sixty-five points: %#v", atCap)
	}
	foundCapGrant := false
	for _, source := range atCap.Sources {
		if source.RequiredBattleLevel != nil && *source.RequiredBattleLevel == 500 {
			foundCapGrant = source.Earned && source.Points == 2 && source.CountedPoints == 2
		}
	}
	if !foundCapGrant {
		t.Fatalf("level 500 two-point source is missing: %#v", atCap.Sources)
	}
}

func TestSpecializationV6DecodeRejectsShiftedPointCheckpointsAndRootLimit(t *testing.T) {
	shifted := loadRealSpecializationTreeForTest(t)
	for index := range shifted.PointBands {
		switch shifted.PointBands[index].ID {
		case "level_200":
			shifted.PointBands[index].PointsPerGrant = 1
		case "level_cap":
			shifted.PointBands[index].PointsPerGrant = 3
		}
	}
	if _, err := specializationTreeFromRaw(shifted); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("shifted level-200 checkpoint was accepted: %v", err)
	}

	tooBroad := loadRealSpecializationTreeForTest(t)
	tooBroad.MaxActiveRootNodes = 4
	if _, err := specializationTreeFromRaw(tooBroad); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("four-root ruleset was accepted: %v", err)
	}
}

func TestSpecializationTreeDecodeRejectsLegacySchemaAndUnknownFields(t *testing.T) {
	valid := map[string]any{
		"id": "tree", "schemaVersion": 1, "rulesetVersion": "v1", "ownerType": "player",
		"nameKey": "tree.name", "descKey": "tree.desc", "i18n": map[string]any{},
		"initialPoints": 0, "maxPoints": 1,
		"unlock":     map[string]any{"battleLevel": 1},
		"pointBands": []any{},
		"reset":      map[string]any{"itemId": "", "count": 0},
		"schools":    []any{},
		"nodes":      []any{},
		"ui":         map[string]any{"layout": "columns", "schoolOrder": []any{}},
	}
	if _, err := specializationTreeFromRaw(valid); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("legacy schema tree was accepted: %v", err)
	}
	v2 := make(map[string]any, len(valid))
	for key, value := range valid {
		v2[key] = value
	}
	v2["schemaVersion"] = 2
	if _, err := specializationTreeFromRaw(v2); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("unknown schema version did not fail closed: %v", err)
	}
	unknown := make(map[string]any, len(valid)+1)
	for key, value := range valid {
		unknown[key] = value
	}
	unknown["futureField"] = true
	if _, err := specializationTreeFromRaw(unknown); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("unknown tree field did not fail closed: %v", err)
	}
	nestedUnknown := make(map[string]any, len(valid))
	for key, value := range valid {
		nestedUnknown[key] = value
	}
	nestedUnknown["unlock"] = map[string]any{"battleLevel": 1, "futureField": true}
	if _, err := specializationTreeFromRaw(nestedUnknown); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("unknown nested tree field did not fail closed: %v", err)
	}
	invalidBand := make(map[string]any, len(valid))
	for key, value := range valid {
		invalidBand[key] = value
	}
	invalidBand["maxPoints"] = 2
	invalidBand["pointBands"] = []any{
		map[string]any{"id": "growth", "fromLevel": 2, "toLevel": 2, "everyLevels": 1, "pointsPerGrant": 1},
		map[string]any{"id": "growth", "fromLevel": 3, "toLevel": 3, "everyLevels": 1, "pointsPerGrant": 1},
	}
	if _, err := specializationTreeFromRaw(invalidBand); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("duplicate point band ids did not fail closed: %v", err)
	}
}

func TestSpecializationV5ExtraTurnAuthoringRequiresOuterCapOrThreeStepCycle(t *testing.T) {
	for _, trigger := range []string{
		"onAfterDealCrit",
		"onAfterDealDamage",
		"onAfterSkillCast",
		"onAfterTakeDamage",
		"onAllyBeforeTakeDamage",
		"onBattleStart",
		"onBeforeDealDamage",
		"onBeforeSkillCast",
		"onBeforeTakeDamage",
		"onKill",
		"onTurnEnd",
		"onTurnStart",
		"onWaveStart",
	} {
		t.Run(trigger, func(t *testing.T) {
			direct := map[string]any{
				"trigger": trigger,
				"effects": []any{map[string]any{"kind": "extra_turn", "targetMode": "self", "turnMode": "full", "dispatchHooks": false}},
				"limits":  map[string]any{"perTurn": 1},
			}
			if err := validateSpecializationV5ExtraTurnSafety([]map[string]any{direct}); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
				t.Fatalf("per-turn-only direct extra action was accepted: %v", err)
			}
			direct["limits"] = map[string]any{"perWave": 1}
			if err := validateSpecializationV5ExtraTurnSafety([]map[string]any{direct}); err != nil {
				t.Fatalf("per-wave direct extra action was rejected: %v", err)
			}
		})
	}
	cycle := map[string]any{
		"trigger": "onAfterSkillCast",
		"effects": []any{map[string]any{
			"kind": "cycle_counter", "maxStacks": 2,
			"atMax": map[string]any{"kind": "extra_turn", "consumeStacks": 2, "targetMode": "self", "turnMode": "full", "dispatchHooks": false},
		}},
	}
	if err := validateSpecializationV5ExtraTurnSafety([]map[string]any{cycle}); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("two-step extra action cycle was accepted: %v", err)
	}
	cycleEffect := cycle["effects"].([]any)[0].(map[string]any)
	cycleEffect["maxStacks"] = 3
	cycleEffect["atMax"].(map[string]any)["consumeStacks"] = 3
	if err := validateSpecializationV5ExtraTurnSafety([]map[string]any{cycle}); err != nil {
		t.Fatalf("three-step extra action cycle was rejected: %v", err)
	}
}

func TestSpecializationRuntimePayloadGrantIDsSurviveReordering(t *testing.T) {
	tree := loadRealSpecializationTreeForTest(t)
	root, found := specializationTreeNode(tree, "strength_root_break_stance")
	if !found || len(root.Levels) == 0 {
		t.Fatal("authoritative strength root is missing")
	}

	cloneNode := func(source specializationNodeSpec) specializationNodeSpec {
		payload, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		var clone specializationNodeSpec
		if err := json.Unmarshal(payload, &clone); err != nil {
			t.Fatal(err)
		}
		return clone
	}
	collectIDs := func(grants []SpecializationRuntimeGrant) map[string]string {
		out := make(map[string]string, len(grants))
		for _, grant := range grants {
			payloadID, _ := grant.Payload["id"].(string)
			out[payloadID] = grant.ID
		}
		return out
	}
	appendNode := func(node specializationNodeSpec) SpecializationBattleContribution {
		out := SpecializationBattleContribution{}
		if err := appendActiveSpecializationNodeContribution(&out, tree, node, len(node.Levels), "battle"); err != nil {
			t.Fatal(err)
		}
		return out
	}

	original := appendNode(root)
	reorderedRoot := cloneNode(root)
	maxLevel := &reorderedRoot.Levels[len(reorderedRoot.Levels)-1]
	foundHookGrant := false
	for grantIndex := range maxLevel.Grants {
		grant := &maxLevel.Grants[grantIndex]
		if grant.ID != "grant_strength_root_break_cycle" {
			continue
		}
		if len(grant.BattleHooks) != 2 {
			t.Fatalf("strength root hook count=%d, want 2", len(grant.BattleHooks))
		}
		grant.BattleHooks[0], grant.BattleHooks[1] = grant.BattleHooks[1], grant.BattleHooks[0]
		foundHookGrant = true
		break
	}
	if !foundHookGrant {
		t.Fatal("authoritative strength root hook grant is missing")
	}
	reordered := appendNode(reorderedRoot)
	if got, want := collectIDs(reordered.Hooks), collectIDs(original.Hooks); !reflect.DeepEqual(got, want) {
		t.Fatalf("hook contribution ids changed after payload reorder: got=%#v want=%#v", got, want)
	}
	for payloadID, runtimeID := range collectIDs(original.Hooks) {
		want, err := specializationRuntimePayloadGrantID(tree.ID, root.ID, "grant_strength_root_break_cycle", map[string]any{"id": payloadID})
		if err != nil {
			t.Fatal(err)
		}
		if runtimeID != want {
			t.Fatalf("hook %s runtime id=%q, want %q", payloadID, runtimeID, want)
		}
	}

	otherNode := cloneNode(root)
	otherNode.ID = "strength_root_break_stance_variant"
	other := appendNode(otherNode)
	for payloadID, runtimeID := range collectIDs(original.Hooks) {
		if otherID := collectIDs(other.Hooks)[payloadID]; otherID == runtimeID {
			t.Fatalf("hook %s collided across nodes at %q", payloadID, runtimeID)
		}
	}

	mutationNode := specializationNodeSpec{
		ID: "stable_mutation_node", NameKey: "stable_mutation_node.name",
		Levels: []specializationLevelSpec{{Level: 1, Cost: 1, Grants: []specializationGrantSpec{{
			ID: "grant_stable_mutations", Kind: "effect_mutation",
			EffectMutations: []map[string]any{{"id": "stable_mutation_a"}, {"id": "stable_mutation_b"}},
		}}}},
	}
	originalMutations := appendNode(mutationNode)
	reorderedMutationNode := cloneNode(mutationNode)
	mutations := reorderedMutationNode.Levels[0].Grants[0].EffectMutations
	mutations[0], mutations[1] = mutations[1], mutations[0]
	reorderedMutations := appendNode(reorderedMutationNode)
	if got, want := collectIDs(reorderedMutations.Mutations), collectIDs(originalMutations.Mutations); !reflect.DeepEqual(got, want) {
		t.Fatalf("mutation contribution ids changed after payload reorder: got=%#v want=%#v", got, want)
	}
}

func TestSpecializationRuntimeIDPartsRejectUnsafeDelimiters(t *testing.T) {
	if !IsValidSpecializationRuntimeIDPart("stable_payload_2") {
		t.Fatal("valid lowercase snake runtime id was rejected")
	}
	invalidIDs := []string{"", "Uppercase", "has-dash", "has:colon", "1leading_digit", " leading_space", "trailing_space "}
	for _, invalidID := range invalidIDs {
		t.Run(fmt.Sprintf("%q", invalidID), func(t *testing.T) {
			if IsValidSpecializationRuntimeIDPart(invalidID) {
				t.Fatalf("unsafe runtime id %q was accepted", invalidID)
			}
			invalidGrant := specializationGrantSpec{
				ID: invalidID, Kind: "battle_hook", BattleHooks: []map[string]any{{"id": "stable_hook"}},
			}
			if err := validateSpecializationGrantShape(invalidGrant); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
				t.Fatalf("unsafe grant id %q was accepted: %v", invalidID, err)
			}

			invalidHook := specializationGrantSpec{
				ID: "stable_grant", Kind: "battle_hook", BattleHooks: []map[string]any{{"id": invalidID}},
			}
			if err := validateSpecializationGrantShape(invalidHook); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
				t.Fatalf("unsafe hook payload id %q was accepted: %v", invalidID, err)
			}

			invalidMutation := specializationGrantSpec{
				ID: "stable_grant", Kind: "effect_mutation", EffectMutations: []map[string]any{{"id": invalidID}},
			}
			if err := validateSpecializationGrantShape(invalidMutation); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
				t.Fatalf("unsafe mutation payload id %q was accepted: %v", invalidID, err)
			}
		})
	}
}

func TestSpecializationV5AuthoringEnvelopeRejectsBroadDamageCoefficientOverflow(t *testing.T) {
	mutation := map[string]any{
		"scope":          "both",
		"effectSelector": map[string]any{"mode": "all"},
		"ops": map[string]any{
			"amountTermsAppend": []any{map[string]any{"attributeKey": "dexterity", "coefficient": 0.041}},
		},
	}
	if err := validateSpecializationV5MutationEnvelope(mutation); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("broad mutation coefficient above 0.04 was accepted: %v", err)
	}
	delete(mutation["ops"].(map[string]any), "amountTermsAppend")
	mutation["ops"].(map[string]any)["amountFormulaTermsAppend"] = []any{map[string]any{"attributeKey": "dexterity", "coefficient": 0.041}}
	if err := validateSpecializationV5MutationEnvelope(mutation); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("amountFormulaTermsAppend alias bypassed the broad coefficient envelope: %v", err)
	}
	delete(mutation["ops"].(map[string]any), "amountFormulaTermsAppend")
	mutation["ops"].(map[string]any)["amountTermsAppend"] = []any{map[string]any{"attributeKey": "dexterity", "coefficient": 0.04}}
	if err := validateSpecializationV5MutationEnvelope(mutation); err != nil {
		t.Fatalf("broad mutation coefficient at the envelope was rejected: %v", err)
	}

	mutation["scope"] = "skill"
	mutation["effectSelector"] = map[string]any{"mode": "kind", "kinds": []any{"damage"}}
	mutation["ops"].(map[string]any)["amountTermsAppend"] = []any{map[string]any{"attributeKey": "dexterity", "coefficient": 0.041}}
	if err := validateSpecializationV5MutationEnvelope(mutation); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("unfiltered kind:damage skill mutation above the wide 0.04 limit was accepted: %v", err)
	}
	mutation["skillCategories"] = []any{"active"}
	if err := validateSpecializationV5MutationEnvelope(mutation); err != nil {
		t.Fatalf("filtered kind:damage skill mutation within the restricted 0.05 limit was rejected: %v", err)
	}
}

func TestSpecializationTreeDecodeAcceptsRealSourceCatalog(t *testing.T) {
	raw, err := os.ReadFile("../../../packages/game-data/source/json/specialization_trees.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Trees []catalog.SpecializationTree `json:"specialization_trees"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Trees) == 0 {
		t.Fatal("real source has no specialization trees")
	}
	for _, rawTree := range source.Trees {
		tree, err := specializationTreeFromRaw(rawTree)
		if err != nil {
			t.Fatalf("real source tree %q rejected: %v", rawTree.ID, err)
		}
		if tree.ID != rawTree.ID || tree.BalanceRevision <= 0 || tree.NameKey == "" || tree.DescKey == "" || len(tree.Schools) == 0 || len(tree.Nodes) == 0 || tree.UI.Layout == "" {
			t.Fatalf("real source display fields were not decoded: %#v", tree)
		}
		root, found := specializationTreeNode(tree, "strength_root_break_stance")
		localized := tree.I18n["zh-CN"].Nodes[root.ID]
		if !found || len(localized.LevelEffectSummary) != len(root.Levels) || len(localized.LevelEffectSummary) < 2 || len(localized.LevelEffectSummary[1]) < 2 {
			t.Fatalf("real source per-rank player copy was truncated: node=%#v copy=%#v", root, localized.LevelEffectSummary)
		}
		if tree.SchemaVersion == 2 {
			if tree.RulesetVersion != specializationRulesetV6 || tree.MaxActiveRootNodes != 3 || len(tree.Keywords) < 20 {
				t.Fatalf("v6 glossary contract was truncated: ruleset=%q roots=%d keywords=%d", tree.RulesetVersion, tree.MaxActiveRootNodes, len(tree.Keywords))
			}
			keywordCopy := tree.I18n["zh-CN"].Keywords["break_momentum"]
			if keywordCopy.Name == "" || keywordCopy.PlayerSummary == "" || keywordCopy.MechanicDetails == "" || keywordCopy.SourceHint == "" {
				t.Fatalf("v6 keyword i18n was truncated: %#v", keywordCopy)
			}
			if len(root.KeywordIDs) == 0 || root.KeywordIDs[0] != "break_momentum" {
				t.Fatalf("v6 node keyword ids were truncated: %#v", root.KeywordIDs)
			}
		}
	}
}

func TestSpecializationBattleContributionDigestIncludesBalanceRevision(t *testing.T) {
	base := SpecializationBattleContribution{
		TreeID:          "tree",
		RulesetVersion:  "specialization-v6",
		BalanceRevision: 7,
		Revision:        3,
		Hooks:           []SpecializationRuntimeGrant{},
		Mutations:       []SpecializationRuntimeGrant{},
		StatModifiers:   []SpecializationStatModifier{},
	}
	first, err := specializationBattleContributionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	base.BalanceRevision++
	second, err := specializationBattleContributionDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || second == "" || first == second {
		t.Fatalf("balance revision must fence battle contribution digests: first=%q second=%q", first, second)
	}
}

func TestSpecializationV6GlossaryLocaleIDsAreClosed(t *testing.T) {
	raw, err := os.ReadFile("../../../packages/game-data/source/json/specialization_trees.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Trees []map[string]any `json:"specialization_trees"`
	}
	if err := json.Unmarshal(raw, &source); err != nil || len(source.Trees) == 0 {
		t.Fatalf("decode real specialization source: trees=%d err=%v", len(source.Trees), err)
	}
	clone := func() map[string]any {
		payload, err := json.Marshal(source.Trees[0])
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if err := json.Unmarshal(payload, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	extraLocaleID := clone()
	locales := extraLocaleID["i18n"].(map[string]any)
	zh := locales["zh-CN"].(map[string]any)
	keywords := zh["keywords"].(map[string]any)
	keywords["unknown_keyword"] = map[string]any{
		"name": "unknown", "playerSummary": "unknown", "mechanicDetails": "unknown", "sourceHint": "unknown",
	}
	if _, err := specializationTreeFromRaw(extraLocaleID); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("locale keyword without a canonical glossary entry was accepted: %v", err)
	}
}

func TestSpecializationConfluenceAnyOfRequiresOneNodeFromEveryGroup(t *testing.T) {
	level := []specializationLevelSpec{{Level: 1, Cost: 1}}
	tree := specializationTreeSpec{
		Nodes: []specializationNodeSpec{
			{ID: "int_a", Levels: level},
			{ID: "int_b", Levels: level},
			{ID: "per_a", Levels: level},
			{ID: "per_b", Levels: level},
			{
				ID:     "crownless_astrolabe",
				Levels: level,
				Requirements: specializationRequirementsSpec{AnyOf: [][]specializationRequirementSpec{
					{{Kind: "node_rank", NodeID: "int_a", MinRank: 1}, {Kind: "node_rank", NodeID: "int_b", MinRank: 1}},
					{{Kind: "node_rank", NodeID: "per_a", MinRank: 1}, {Kind: "node_rank", NodeID: "per_b", MinRank: 1}},
				}},
			},
		},
	}
	target, _ := specializationTreeNode(tree, "crownless_astrolabe")
	valid := []SpecializationAllocation{{NodeID: "int_a", Rank: 1, PaidCost: 1}, {NodeID: "per_b", Rank: 1, PaidCost: 1}}
	if reasons := specializationNodeUnmetReasons(tree, target, valid, nil, 10); containsSpecializationReason(reasons, "REQUIREMENT_ANY_OF") {
		t.Fatalf("one technique from each school should satisfy confluence, reasons=%v", reasons)
	}
	invalid := []SpecializationAllocation{{NodeID: "int_a", Rank: 1, PaidCost: 1}, {NodeID: "int_b", Rank: 1, PaidCost: 1}}
	if reasons := specializationNodeUnmetReasons(tree, target, invalid, nil, 10); !containsSpecializationReason(reasons, "REQUIREMENT_ANY_OF") {
		t.Fatalf("missing perception group should block confluence, reasons=%v", reasons)
	}
}

func TestSpecializationNodeStateRequiresConfiguredParentRank(t *testing.T) {
	rootLevels := make([]specializationLevelSpec, 6)
	for index := range rootLevels {
		rootLevels[index] = specializationLevelSpec{Level: index + 1, Cost: 1}
	}
	tree := specializationTreeSpec{
		ID: playerCombatSpecializationTreeID, InitialPoints: 40, MaxPoints: 40,
		Unlock: specializationUnlockSpec{BattleLevel: 10},
		Nodes: []specializationNodeSpec{
			{ID: "root", Kind: "root", SchoolIDs: []string{"strength"}, Levels: rootLevels},
			{ID: "tech", Kind: "technique", SchoolIDs: []string{"strength"}, Requirements: specializationRequirementsSpec{All: []specializationRequirementSpec{{Kind: "node_rank", NodeID: "root", MinRank: 6}}}, Levels: []specializationLevelSpec{{Level: 1, Cost: 2}}},
		},
	}
	rootFive := []SpecializationAllocation{{NodeID: "root", Rank: 5, PaidCost: 5}}
	status := buildSpecializationStatus(tree, 1, 500, 0, nil, rootFive)
	tech := specializationNodeStateByID(status.NodeStates, "tech")
	if tech == nil || tech.CanAllocate || !containsSpecializationReason(tech.UnmetReasonCodes, "REQUIREMENT_NODE_RANK") {
		t.Fatalf("technique must remain locked at parent rank 5/6: %#v", tech)
	}
	rootSix := []SpecializationAllocation{{NodeID: "root", Rank: 6, PaidCost: 6}}
	status = buildSpecializationStatus(tree, 2, 500, 0, nil, rootSix)
	tech = specializationNodeStateByID(status.NodeStates, "tech")
	if tech == nil || !tech.CanAllocate || len(tech.UnmetReasonCodes) != 0 {
		t.Fatalf("technique must unlock exactly at parent rank 6/6: %#v", tech)
	}
}

func TestSpecializationActiveRootLimitBlocksFourthRouteButAllowsRootUpgrades(t *testing.T) {
	rootLevels := []specializationLevelSpec{{Level: 1, Cost: 1}, {Level: 2, Cost: 1}}
	tree := specializationTreeSpec{
		RulesetVersion:     specializationRulesetV6,
		MaxPoints:          65,
		MaxActiveRootNodes: 3,
		Nodes: []specializationNodeSpec{
			{ID: "root_a", Kind: "root", Levels: rootLevels},
			{ID: "root_b", Kind: "root", Levels: rootLevels},
			{ID: "root_c", Kind: "root", Levels: rootLevels},
			{ID: "root_d", Kind: "root", Levels: rootLevels},
		},
	}
	threeRoots := []SpecializationAllocation{
		{NodeID: "root_a", Rank: 1, PaidCost: 1},
		{NodeID: "root_b", Rank: 1, PaidCost: 1},
		{NodeID: "root_c", Rank: 1, PaidCost: 1},
	}
	fourth, _ := specializationTreeNode(tree, "root_d")
	if reasons := specializationNodeUnmetReasons(tree, fourth, threeRoots, nil, 65); !containsSpecializationReason(reasons, "ACTIVE_ROOT_LIMIT") {
		t.Fatalf("fourth root route must be blocked, reasons=%v", reasons)
	}
	status := buildSpecializationStatus(tree, 3, 500, 65, nil, threeRoots)
	fourthState := specializationNodeStateByID(status.NodeStates, "root_d")
	if fourthState == nil || fourthState.CanAllocate || !containsSpecializationReason(fourthState.UnmetReasonCodes, "ACTIVE_ROOT_LIMIT") {
		t.Fatalf("fourth root route must be exposed as locked to clients: %#v", fourthState)
	}
	existing, _ := specializationTreeNode(tree, "root_a")
	if reasons := specializationNodeUnmetReasons(tree, existing, threeRoots, nil, 65); containsSpecializationReason(reasons, "ACTIVE_ROOT_LIMIT") {
		t.Fatalf("an active root route must remain upgradeable, reasons=%v", reasons)
	}
	fourRoots := append(append([]SpecializationAllocation(nil), threeRoots...), SpecializationAllocation{NodeID: "root_d", Rank: 1, PaidCost: 1})
	if err := validateCurrentSpecializationAllocations(tree, fourRoots); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("persisted fourth root route was accepted: %v", err)
	}
}

func TestSpecializationMultiRankUsesCurrentCompleteLevelAndCumulativePaidCost(t *testing.T) {
	level := func(rank, cost int, value float64, marker string, accessoryDelta int) specializationLevelSpec {
		return specializationLevelSpec{
			Level: rank,
			Cost:  cost,
			Grants: []specializationGrantSpec{
				{ID: "stats", Kind: "stat_modifier", Scope: "battle", Modifiers: []SpecializationStatModifier{{AttributeKey: "strength", Operation: "add", Value: value}}},
				{ID: "hook", Kind: "battle_hook", BattleHooks: []map[string]any{{"id": "core_hook", "rankMarker": marker}}},
				{ID: "mutation", Kind: "effect_mutation", EffectMutations: []map[string]any{{"id": "core_mutation", "rankMarker": marker}}},
				{ID: "slots", Kind: "equipment_slot_capacity", EquipType: "battle", Deltas: map[string]int{"accessory": accessoryDelta}},
			},
		}
	}
	node := specializationNodeSpec{
		ID:      "ranked_root",
		Kind:    "root",
		NameKey: "ranked_root.name",
		Levels: []specializationLevelSpec{
			level(1, 1, 1, "rank1", 1),
			level(2, 2, 2, "rank2", 2),
			level(3, 3, 3, "rank3", 1),
		},
	}
	tree := specializationTreeSpec{ID: "tree", MaxPoints: 10, Nodes: []specializationNodeSpec{node}}
	out := SpecializationBattleContribution{}
	if err := appendActiveSpecializationNodeContribution(&out, tree, node, 2, "battle"); err != nil {
		t.Fatal(err)
	}
	if len(out.StatModifiers) != 1 || out.StatModifiers[0].Value != 2 || len(out.Hooks) != 1 || out.Hooks[0].Payload["rankMarker"] != "rank2" || len(out.Mutations) != 1 || out.Mutations[0].Payload["rankMarker"] != "rank2" {
		t.Fatalf("rank 2 must expose only its complete level snapshot: %#v", out)
	}
	deltas, err := specializationCapacityDeltas(tree, []SpecializationAllocation{{NodeID: node.ID, Rank: 2, PaidCost: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if deltas["accessory"] != 2 {
		t.Fatalf("rank 2 topology delta=%d, want current snapshot 2 instead of cumulative 3", deltas["accessory"])
	}
	paidCost, err := specializationCanonicalPaidCost(node, 2)
	if err != nil || paidCost != 3 {
		t.Fatalf("rank 2 paidCost=%d err=%v, want cumulative 1+2", paidCost, err)
	}
	prospective := prospectiveAllocationsWithRank([]SpecializationAllocation{{NodeID: node.ID, Rank: 1, PaidCost: 1}}, node.ID, 2, node.Levels[1].Cost)
	if len(prospective) != 1 || prospective[0].Rank != 2 || prospective[0].PaidCost != 3 {
		t.Fatalf("rank upgrade did not preserve cumulative paid cost: %#v", prospective)
	}
	status := buildSpecializationStatus(tree, 4, 1, 10, nil, prospective)
	if len(status.NodeStates) != 1 || status.NodeStates[0].Rank != 2 || status.NodeStates[0].MaxRank != 3 || status.NodeStates[0].NextCost != 3 || !status.NodeStates[0].CanAllocate {
		t.Fatalf("multi-rank node state is incorrect: %#v", status.NodeStates)
	}
}

func TestSpecializationPureSchoolGateUsesCumulativeRanks(t *testing.T) {
	tree := specializationTreeSpec{
		ID:             playerCombatSpecializationTreeID,
		RulesetVersion: specializationRulesetV6,
		MaxPoints:      40,
		Nodes: []specializationNodeSpec{
			{
				ID: "strength_root", Kind: "root", SchoolIDs: []string{"strength"},
				Levels: []specializationLevelSpec{{Level: 1, Cost: 1}, {Level: 2, Cost: 2}, {Level: 3, Cost: 1}},
			},
			{
				ID: "strength_tech", Kind: "technique", SchoolIDs: []string{"strength"},
				Requirements: specializationRequirementsSpec{All: []specializationRequirementSpec{{Kind: "node_rank", NodeID: "strength_root", MinRank: 1}}},
				Levels:       []specializationLevelSpec{{Level: 1, Cost: 2}},
			},
			{
				ID: "strength_terminal", Kind: "terminal", SchoolIDs: []string{"strength"},
				Requirements: specializationRequirementsSpec{All: []specializationRequirementSpec{
					{Kind: "node_rank", NodeID: "strength_tech", MinRank: 1},
					{Kind: "pure_school_points_gte", SchoolID: "strength", Points: 6},
				}},
				Levels: []specializationLevelSpec{{Level: 1, Cost: 4}},
			},
		},
	}
	terminal, _ := specializationTreeNode(tree, "strength_terminal")
	shallow := []SpecializationAllocation{
		{NodeID: "strength_root", Rank: 1, PaidCost: 1},
		{NodeID: "strength_tech", Rank: 1, PaidCost: 2},
	}
	if reasons := specializationNodeUnmetReasons(tree, terminal, shallow, nil, 40); !containsSpecializationReason(reasons, "REQUIREMENT_PURE_SCHOOL_POINTS_GTE") {
		t.Fatalf("three shallow pure-school points should not unlock a six-point terminal gate: %v", reasons)
	}
	deep := []SpecializationAllocation{
		{NodeID: "strength_root", Rank: 3, PaidCost: 4},
		{NodeID: "strength_tech", Rank: 1, PaidCost: 2},
	}
	if reasons := specializationNodeUnmetReasons(tree, terminal, deep, nil, 40); containsSpecializationReason(reasons, "REQUIREMENT_PURE_SCHOOL_POINTS_GTE") {
		t.Fatalf("cumulative multi-rank paidCost should satisfy the pure-school gate: %v", reasons)
	}
}

func containsSpecializationReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func specializationNodeStateByID(states []SpecializationNodeState, nodeID string) *SpecializationNodeState {
	for index := range states {
		if states[index].NodeID == nodeID {
			return &states[index]
		}
	}
	return nil
}

func TestApplySpecializationStatModifiersIsOrdered(t *testing.T) {
	values := map[string]float64{"strength": 10}
	err := applySpecializationStatModifiers(values, []SpecializationStatModifier{
		{AttributeKey: "strength", Operation: "add", Value: 2},
		{AttributeKey: "strength", Operation: "multiply", Value: 1.5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if values["strength"] != 18 {
		t.Fatalf("strength=%v, want 18", values["strength"])
	}
}

func TestSpecializationTreeRejectsUnknownOrMalformedGrantShapes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		grant specializationGrantSpec
	}{
		{name: "unknown kind", grant: specializationGrantSpec{ID: "grant", Kind: "future_grant"}},
		{name: "capacity wrong equip type", grant: specializationGrantSpec{ID: "grant", Kind: "equipment_slot_capacity", EquipType: "work", Deltas: map[string]int{"head": -1}}},
		{name: "capacity empty deltas", grant: specializationGrantSpec{ID: "grant", Kind: "equipment_slot_capacity", EquipType: "battle"}},
		{name: "capacity unknown family", grant: specializationGrantSpec{ID: "grant", Kind: "equipment_slot_capacity", EquipType: "battle", Deltas: map[string]int{"tail": 1}}},
		{name: "capacity zero delta", grant: specializationGrantSpec{ID: "grant", Kind: "equipment_slot_capacity", EquipType: "battle", Deltas: map[string]int{"head": 0}}},
		{name: "empty battle hook", grant: specializationGrantSpec{ID: "grant", Kind: "battle_hook"}},
		{name: "empty mutation", grant: specializationGrantSpec{ID: "grant", Kind: "effect_mutation"}},
		{name: "invalid stat scope", grant: specializationGrantSpec{ID: "grant", Kind: "stat_modifier", Scope: "account", Modifiers: []SpecializationStatModifier{{AttributeKey: "strength", Operation: "add", Value: 1}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSpecializationGrantShape(tc.grant); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
				t.Fatalf("malformed grant did not fail closed: err=%v grant=%#v", err, tc.grant)
			}
		})
	}

	valid := []specializationGrantSpec{
		{ID: "stat", Kind: "stat_modifier", Scope: "character", Modifiers: []SpecializationStatModifier{{AttributeKey: "strength", Operation: "add", Value: 1}}},
		{ID: "hook", Kind: "battle_hook", BattleHooks: []map[string]any{{"id": "hook"}}},
		{ID: "mutation", Kind: "effect_mutation", EffectMutations: []map[string]any{{"id": "mutation"}}},
		{ID: "capacity", Kind: "equipment_slot_capacity", EquipType: "battle", Deltas: map[string]int{"accessory": 1}},
	}
	for _, grant := range valid {
		if err := validateSpecializationGrantShape(grant); err != nil {
			t.Fatalf("valid %s grant rejected: %v", grant.Kind, err)
		}
	}

	coreHook := specializationGrantSpec{ID: "core", Kind: "battle_hook", BattleHooks: []map[string]any{{"id": "core"}}}
	if err := validateSpecializationGrantSnapshotRetention(coreHook, specializationGrantSpec{
		ID: "core", Kind: "battle_hook", BattleHooks: []map[string]any{{"id": "different_hook"}},
	}); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("higher rank that replaces a stable nested hook id did not fail closed: %v", err)
	}
	duplicateHooks := specializationGrantSpec{
		ID: "core", Kind: "battle_hook", BattleHooks: []map[string]any{{"id": "same"}, {"id": "same"}},
	}
	if err := validateSpecializationGrantShape(duplicateHooks); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("duplicate stable hook ids did not fail closed: %v", err)
	}
}

func TestLoadoutApplicationLocksUserBeforeTopologyValidation(t *testing.T) {
	raw, err := os.ReadFile("store_loadouts.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, functionName := range []string{"ApplyBattleLoadout", "applyWorkLoadoutTx"} {
		start := strings.Index(text, "func (s *Store) "+functionName)
		if start < 0 {
			t.Fatalf("%s not found", functionName)
		}
		body := text[start:]
		if next := strings.Index(body[1:], "\nfunc "); next >= 0 {
			body = body[:next+1]
		}
		lockAt := strings.Index(body, "lockUserTx")
		validateAt := strings.Index(body, "validateLoadoutEquipmentTx")
		if lockAt < 0 || validateAt < 0 || lockAt > validateAt {
			t.Fatalf("%s must lock the user before topology/equipment validation", functionName)
		}
	}
}

func TestBuildSpecializationStatusCapsFormulaAndBonusAtTreeMaximum(t *testing.T) {
	tree := specializationTreeSpec{
		InitialPoints: 20,
		MaxPoints:     24,
		Unlock:        specializationUnlockSpec{BattleLevel: 1},
	}
	status := buildSpecializationStatus(tree, 3, 1, 10, nil, nil)
	if status.FormulaPoints != 20 || status.BonusPoints != 10 || status.TotalPoints != 24 || status.AvailablePoints != 24 {
		t.Fatalf("unexpected capped accounting: %#v", status)
	}
	if !status.PointBreakdown.AtCap || status.PointBreakdown.CountedRewardPoints != 4 || status.PointBreakdown.CappedPoints != 6 {
		t.Fatalf("status is missing authoritative point source accounting: %#v", status.PointBreakdown)
	}
}

func TestResolvePlayerBattleSlotProfileSupportsSpecializationTopologies(t *testing.T) {
	profile, err := ResolvePlayerBattleSlotProfile(map[string]int{"head": -1, "accessory": 2})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Capacities["head"] != 0 || profile.Capacities["accessory"] != 6 || profile.Enabled("head") || !profile.Enabled("accessory_6") {
		t.Fatalf("unexpected crownless topology: %#v", profile)
	}
	profile, err = ResolvePlayerBattleSlotProfile(map[string]int{"accessory": -2, "right_hand": 1})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Capacities["accessory"] != 2 || profile.Capacities["right_hand"] != 2 || !profile.Enabled("right_hand_2") || profile.Enabled("accessory_3") {
		t.Fatalf("unexpected three-armed topology: %#v", profile)
	}
	profile, err = ResolvePlayerBattleSlotProfile(map[string]int{"head": -1, "accessory": 0, "right_hand": 1})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Capacities["head"] != 0 || profile.Capacities["accessory"] != 4 || profile.Capacities["right_hand"] != 2 || profile.Enabled("head") || !profile.Enabled("right_hand_2") || !profile.Enabled("accessory_4") {
		t.Fatalf("unexpected combined terminal topology: %#v", profile)
	}
	if _, err := ResolvePlayerBattleSlotProfile(map[string]int{"right_hand": 2}); err == nil {
		t.Fatal("right-hand capacity above two must fail closed")
	}
}

func TestSpecializationStateResetReasonAllowsAdditiveNodesInCompatibleRuleset(t *testing.T) {
	tree := specializationTreeSpec{
		RulesetVersion: specializationRulesetV6,
		MaxPoints:      10,
		Nodes: []specializationNodeSpec{
			{
				ID:     "node",
				Levels: []specializationLevelSpec{{Level: 1, Cost: 2}},
			},
			{
				ID:     "new_additive_node",
				Levels: []specializationLevelSpec{{Level: 1, Cost: 1}},
			},
		},
	}
	valid := []SpecializationAllocation{{NodeID: "node", Rank: 1, PaidCost: 2}}
	if got := specializationStateResetReason(tree, specializationRulesetV6, valid); got != "" {
		t.Fatalf("additive node invalidated existing allocation: %q", got)
	}
	if got := specializationStateResetReason(tree, "specialization-v5", valid); got != "ruleset_changed" {
		t.Fatalf("incompatible ruleset reset reason=%q", got)
	}
	if got := specializationStateResetReason(tree, specializationRulesetV6, []SpecializationAllocation{{NodeID: "node", Rank: 1, PaidCost: 1}}); got != "allocation_invalid" {
		t.Fatalf("repriced allocation reset reason=%q", got)
	}
	if got := specializationStateResetReason(tree, specializationRulesetV6, nil); got != "" {
		t.Fatalf("new tree empty state unexpectedly reset: %q", got)
	}
}

func TestRulesetResetImpactReturnsEquipmentFromRemovedExtraSlot(t *testing.T) {
	store := &Store{gameData: catalog.Data{EquipmentTemplates: []catalog.EquipmentTemplate{{
		ID: "single_blade", EquipType: "battle", Slot: "right_hand",
	}}}}
	baseProfile := BasePlayerBattleSlotProfile()
	previewTx := &specializationEquipmentTx{rows: &specializationEquipmentRows{rows: [][]any{{int64(202), "single_blade", "right_hand_2"}}}}
	impact, err := store.specializationEquipmentImpactTx(
		context.Background(), previewTx, 42, playerCombatSpecializationTreeID, 7,
		"ruleset_reset", "", baseProfile, baseProfile,
	)
	if err != nil {
		t.Fatal(err)
	}
	impact.DisabledSlots = specializationRulesetResetDisabledSlots(impact, baseProfile)
	if !reflect.DeepEqual(impact.DisabledSlots, []string{"right_hand_2"}) || len(impact.AutoUnequipped) != 1 || impact.AutoUnequipped[0].InstanceID != 202 {
		t.Fatalf("ruleset reset did not project extra-slot equipment return: %#v", impact)
	}
	commitTx := &specializationEquipmentTx{row: specializationBoolRow(false)}
	if err := store.applySpecializationEquipmentImpactTx(context.Background(), commitTx, 42, impact); err != nil {
		t.Fatal(err)
	}
	if len(commitTx.statements) != 2 || !strings.Contains(commitTx.statements[0], "DELETE FROM player_equip_slots") || !strings.Contains(commitTx.statements[1], "SET state = 'inventory'") {
		t.Fatalf("ruleset reset did not remove the whole item and return it to inventory: %#v", commitTx.statements)
	}
}

func TestRulesetResetTransactionOrdersStateAndEquipmentRepairAtomically(t *testing.T) {
	raw, err := os.ReadFile("store_specializations.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	start := strings.Index(body, "func (s *Store) resetIncompatibleSpecializationStateTx")
	if start < 0 {
		t.Fatal("ruleset reset owner function not found")
	}
	end := strings.Index(body[start:], "\nfunc (s *Store) specializationStateTx")
	if end < 0 {
		t.Fatal("ruleset reset owner function end not found")
	}
	body = body[start : start+end]
	clearAllocationsAt := strings.Index(body, "replaceSpecializationAllocationsTx")
	updateRulesetAt := strings.Index(body, "SET ruleset_version")
	returnEquipmentAt := strings.Index(body, "applySpecializationEquipmentImpactTx")
	recomputeAt := strings.Index(body, "onPlayerAttributesChangedTx")
	reconcileAt := strings.Index(body, "reconcileUnmetEquipmentStatusReqTx")
	disableSkillsAt := strings.Index(body, "disableSpecializationSkillsTx")
	persistNoticeAt := strings.LastIndex(body, "last_ruleset_reset_jsonb")
	if clearAllocationsAt < 0 || updateRulesetAt < clearAllocationsAt || returnEquipmentAt < updateRulesetAt || recomputeAt < returnEquipmentAt || reconcileAt < recomputeAt || disableSkillsAt < reconcileAt || persistNoticeAt < disableSkillsAt {
		t.Fatalf("ruleset reset no longer clears allocations, switches rules, returns equipment, recomputes, reconciles, disables skills, and records the notice in order")
	}
}

func TestSpecializationResetTopologyPreviewAndCommitUnequipsWholeSecondHandItem(t *testing.T) {
	tree := specializationTreeSpec{
		ID: playerCombatSpecializationTreeID,
		Nodes: []specializationNodeSpec{
			{
				ID: "strength_terminal_three_armed",
				Levels: []specializationLevelSpec{{Level: 1, Cost: 4, Grants: []specializationGrantSpec{{
					ID: "three_armed_slots", Kind: "equipment_slot_capacity", EquipType: "battle",
					Deltas: map[string]int{"accessory": -2, "right_hand": 1},
				}}}},
			},
			{
				ID: "confluence_intelligence_perception_crownless_astrolabe",
				Levels: []specializationLevelSpec{{Level: 1, Cost: 3, Grants: []specializationGrantSpec{{
					ID: "crownless_slots", Kind: "equipment_slot_capacity", EquipType: "battle",
					Deltas: map[string]int{"head": -1, "accessory": 2},
				}}}},
			},
		},
	}
	store := &Store{gameData: catalog.Data{EquipmentTemplates: []catalog.EquipmentTemplate{{
		ID: "single_blade", EquipType: "battle", Slot: "right_hand",
	}}}}
	threeArmed := []SpecializationAllocation{{NodeID: "strength_terminal_three_armed", Rank: 1, PaidCost: 4}}
	before, err := store.resolvePlayerBattleSlotProfileForAllocations(tree, threeArmed)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.resolvePlayerBattleSlotProfileForAllocations(tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.Capacities["right_hand"] != 2 || !before.Enabled("right_hand_2") || after.Capacities["right_hand"] != 1 || after.Enabled("right_hand_2") {
		t.Fatalf("reset topology projection before=%#v after=%#v", before.Capacities, after.Capacities)
	}
	previewTx := &specializationEquipmentTx{rows: &specializationEquipmentRows{rows: [][]any{{int64(202), "single_blade", "right_hand_2"}}}}
	impact, err := store.specializationEquipmentImpactTx(context.Background(), previewTx, 42, tree.ID, 7, "reset", "", before, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(previewTx.statements) != 0 {
		t.Fatalf("impact preview mutated equipment: %#v", previewTx.statements)
	}
	if len(impact.DisabledSlots) != 1 || impact.DisabledSlots[0] != "right_hand_2" || len(impact.AutoUnequipped) != 1 || impact.AutoUnequipped[0].InstanceID != 202 || !reflect.DeepEqual(impact.AutoUnequipped[0].FromSlots, []string{"right_hand_2"}) || strings.TrimSpace(impact.Digest) == "" {
		t.Fatalf("reset preview did not report the second-hand item: %#v", impact)
	}
	commitTx := &specializationEquipmentTx{row: specializationBoolRow(false)}
	if err := store.applySpecializationEquipmentImpactTx(context.Background(), commitTx, 42, impact); err != nil {
		t.Fatal(err)
	}
	if len(commitTx.statements) != 2 || !strings.Contains(commitTx.statements[0], "DELETE FROM player_equip_slots") || !strings.Contains(commitTx.statements[0], "instance_id = $2") || !strings.Contains(commitTx.statements[1], "SET state = 'inventory'") {
		t.Fatalf("reset must remove every slot for the instance and return it to inventory: %#v", commitTx.statements)
	}
	if !reflect.DeepEqual(commitTx.arguments[0], []any{int64(42), int64(202)}) || !reflect.DeepEqual(commitTx.arguments[1], []any{int64(202), int64(42)}) {
		t.Fatalf("whole-item unequip arguments=%#v", commitTx.arguments)
	}

	combined := []SpecializationAllocation{
		{NodeID: "strength_terminal_three_armed", Rank: 1, PaidCost: 4},
		{NodeID: "confluence_intelligence_perception_crownless_astrolabe", Rank: 1, PaidCost: 3},
	}
	combinedProfile, err := store.resolvePlayerBattleSlotProfileForAllocations(tree, combined)
	if err != nil {
		t.Fatal(err)
	}
	if combinedProfile.Capacities["head"] != 0 || combinedProfile.Capacities["accessory"] != 4 || combinedProfile.Capacities["right_hand"] != 2 || !combinedProfile.Enabled("right_hand_2") {
		t.Fatalf("combined topology before reset=%#v", combinedProfile.Capacities)
	}
	disabled, enabled := equipmentSlotCapacityDelta(combinedProfile, after)
	if !reflect.DeepEqual(disabled, []string{"right_hand_2"}) || !reflect.DeepEqual(enabled, []string{"head"}) {
		t.Fatalf("combined reset slot delta disabled=%v enabled=%v", disabled, enabled)
	}
}

func TestPlayerSpecializationSchemaFilesStayAligned(t *testing.T) {
	paths := []string{
		"../../schema/init.sql",
		"../../schema/migrations/20260826_player_specializations.sql",
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(raw)
		for _, required := range []string{"player_specialization_states", "player_specialization_allocations", "player_specialization_ledger", "ruleset_version TEXT", "formula_points", "bonus_points", "spent_points", "last_ruleset_reset_jsonb", "idempotency_key", "result_jsonb"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s missing %q", path, required)
			}
		}
		if strings.Contains(text, "UNIQUE (user_id, tree_id, revision)") || strings.Contains(text, "CHECK(revision > 0)") {
			t.Fatalf("%s forbids idempotent no-op reset ledger rows at revision zero or repeated revision", path)
		}
		if strings.Contains(text, "idx_player_specialization_allocations_tree") {
			t.Fatalf("%s duplicates the allocation primary-key index", path)
		}
	}
	initRaw, err := os.ReadFile("../../schema/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(initRaw), "pending_migration_receipt_jsonb") {
		t.Fatal("prelaunch specialization schema still carries a legacy migration receipt column")
	}
	down, err := os.ReadFile("../../schema/migrations/20260826_player_specializations_down.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(down)
	ledger := strings.Index(text, "player_specialization_ledger")
	allocations := strings.Index(text, "player_specialization_allocations")
	states := strings.Index(text, "player_specialization_states")
	if ledger < 0 || allocations <= ledger || states <= allocations {
		t.Fatalf("down migration dependency order is invalid: %s", text)
	}
}

func TestNativeUpdateLeavesSchemaMigrationsManual(t *testing.T) {
	updateScript, err := os.ReadFile("../../../infra/scripts/update-linux-native-split.sh")
	if err != nil {
		t.Fatalf("read native update script: %v", err)
	}
	text := string(updateScript)
	for _, forbidden := range []string{
		"apply_player_specialization_schema",
		"apply_player_username_history_schema",
		"migrate-player-specializations-native.sh",
		"migrate-player-username-history-native.sh",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("native update must not run manual schema migration path %q", forbidden)
		}
	}

	migrationScript, err := os.ReadFile("../../../infra/scripts/migrate-player-specializations-native.sh")
	if err != nil {
		t.Fatalf("read specialization migration script: %v", err)
	}
	migrationText := string(migrationScript)
	for _, required := range []string{
		"20260826_player_specializations.sql",
		"20260828_battle_attr_exp_jobs.sql",
		"go run ./cmd/schema_apply",
	} {
		if !strings.Contains(migrationText, required) {
			t.Fatalf("specialization migration script missing %q", required)
		}
	}
}

func TestNativeUpdatePreservesBattleWorkerDatabasePoolConfig(t *testing.T) {
	updateScript, err := os.ReadFile("../../../infra/scripts/update-linux-native-split.sh")
	if err != nil {
		t.Fatalf("read native update script: %v", err)
	}
	text := string(updateScript)
	for _, forbidden := range []string{
		"configure_battle_worker_db_pool",
		"BATTLE_DB_POOL_MAX_CONNS",
		"BATTLE_DB_POOL_MIN_CONNS",
		"pool_max_conn_lifetime",
		"pool_max_conn_idle_time",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("native update must preserve explicit battle-worker DB pool config, found %q", forbidden)
		}
	}
	for _, required := range []string{
		"build_battle_worker_binary",
		"restart_battle_worker",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("native update lost battle-worker update step %q", required)
		}
	}
}
