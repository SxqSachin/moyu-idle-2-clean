package store

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"mysecondwebgame/server/internal/catalog"
	"mysecondwebgame/server/internal/config"

	"github.com/jackc/pgx/v5"
)

const skillForgeRecentHistoryLimit = 50

type SkillForgeDraft struct {
	ID                    string            `json:"id"`
	CustomSkillID         string            `json:"customSkillId,omitempty"`
	SourceKind            string            `json:"sourceKind"`
	SourceSkillID         string            `json:"sourceSkillId,omitempty"`
	SourceSkillbookItemID string            `json:"sourceSkillbookItemId,omitempty"`
	SourceSkillbookCount  int64             `json:"sourceSkillbookCount,omitempty"`
	Status                string            `json:"status"`
	SourceDefinitionHash  string            `json:"sourceDefinitionHash,omitempty"`
	Graph                 SkillForgeGraphV1 `json:"graph"`
	RevisionNo            int64             `json:"revisionNo"`
	PendingRevisionID     string            `json:"pendingRevisionId,omitempty"`
	CreatedAt             time.Time         `json:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt"`
}

type PlayerCustomSkill struct {
	ID                   string         `json:"id"`
	SourceKind           string         `json:"sourceKind"`
	SourceSkillID        string         `json:"sourceSkillId,omitempty"`
	SourceDefinitionHash string         `json:"sourceDefinitionHash,omitempty"`
	CurrentRevisionID    string         `json:"currentRevisionId,omitempty"`
	Archived             bool           `json:"archived"`
	RulesetVersion       int            `json:"rulesetVersion"`
	Usable               bool           `json:"usable"`
	DisabledReason       string         `json:"disabledReason,omitempty"`
	CompiledSkill        map[string]any `json:"compiledSkill,omitempty"`
	Budget               map[string]any `json:"budget,omitempty"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

type SkillForgeRevision struct {
	ID                    string                     `json:"id"`
	CustomSkillID         string                     `json:"customSkillId"`
	DraftID               string                     `json:"draftId,omitempty"`
	RevisionNo            int64                      `json:"revisionNo"`
	SourceSkillID         string                     `json:"sourceSkillId,omitempty"`
	SourceDefinitionHash  string                     `json:"sourceDefinitionHash,omitempty"`
	Graph                 SkillForgeGraphV1          `json:"graph"`
	CompiledSkill         map[string]any             `json:"compiledSkill"`
	Recipe                map[string]int64           `json:"recipe,omitempty"`
	RequestedRecipe       map[string]int64           `json:"requestedRecipe,omitempty"`
	ConsumedRecipe        map[string]int64           `json:"consumedRecipe,omitempty"`
	RefundedRecipe        map[string]int64           `json:"refundedRecipe,omitempty"`
	SourceSkillbookItemID string                     `json:"sourceSkillbookItemId,omitempty"`
	SourceSkillbookCount  int64                      `json:"sourceSkillbookCount,omitempty"`
	Budget                map[string]any             `json:"budget"`
	Rolls                 []SkillForgeParameterRange `json:"rolls"`
	Failed                bool                       `json:"failed"`
	ResultStatus          string                     `json:"resultStatus"`
	CompilerVersion       int                        `json:"compilerVersion"`
	RulesetVersion        int                        `json:"rulesetVersion"`
	CreatedAt             time.Time                  `json:"createdAt"`
	ResolvedAt            *time.Time                 `json:"resolvedAt,omitempty"`
	ForgedSkillbook       *ForgedSkillbookSummary    `json:"forgedSkillbook,omitempty"`
}

type ForgedSkillbookSummary struct {
	ID             string         `json:"id"`
	InstanceID     int64          `json:"instanceId,omitempty"`
	CustomSkillID  string         `json:"customSkillId"`
	RevisionID     string         `json:"revisionId"`
	SourceSkillID  string         `json:"sourceSkillId"`
	Status         string         `json:"status"`
	RulesetVersion int            `json:"rulesetVersion"`
	Usable         bool           `json:"usable"`
	TargetKittyID  int64          `json:"targetKittyId,omitempty"`
	NameParams     map[string]any `json:"nameParams,omitempty"`
	CompiledSkill  map[string]any `json:"compiledSkill,omitempty"`
	Budget         map[string]any `json:"budget,omitempty"`
}

type SkillForgeInventoryEntry struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

type SkillForgeSourceBook struct {
	ItemID   string `json:"itemId"`
	SkillID  string `json:"skillId"`
	Rarity   string `json:"rarity"`
	IconID   string `json:"iconId"`
	Quantity int64  `json:"quantity"`
}

type SkillForgeState struct {
	Modules          any                        `json:"modules"`
	Limits           any                        `json:"limits"`
	Derived          SkillForgeDerived          `json:"derived"`
	Inventory        []SkillForgeInventoryEntry `json:"inventory"`
	Drafts           []SkillForgeDraft          `json:"drafts"`
	Skills           []PlayerCustomSkill        `json:"skills"`
	PendingRevisions []SkillForgeRevision       `json:"pendingRevisions"`
	OwnedSourceIDs   []string                   `json:"ownedSourceIds"`
	SourceSkillbooks []SkillForgeSourceBook     `json:"sourceSkillbooks"`
}

type CreateSkillForgeDraftInput struct {
	SourceKind            string
	SourceSkillID         string
	SourceSkillbookItemID string
	SourceSkillbookCount  int64
	Graph                 SkillForgeGraphV1
}

func (s *Store) SkillForgeState(ctx context.Context, userID int64) (SkillForgeState, error) {
	derived, err := s.skillForgeDerived(ctx, userID)
	if err != nil {
		return SkillForgeState{}, fmt.Errorf("resolve skill forge attributes: %w", err)
	}
	if err := s.normalizeLegacySkillForgeBookCounts(ctx, userID); err != nil {
		return SkillForgeState{}, fmt.Errorf("normalize skill forge book counts: %w", err)
	}
	drafts, err := s.ListSkillForgeDrafts(ctx, userID)
	if err != nil {
		return SkillForgeState{}, fmt.Errorf("list skill forge drafts: %w", err)
	}
	skills, err := s.ListPlayerCustomSkills(ctx, userID, false)
	if err != nil {
		return SkillForgeState{}, fmt.Errorf("list custom skills: %w", err)
	}
	pending, err := s.listSkillForgePendingRevisions(ctx, userID)
	if err != nil {
		return SkillForgeState{}, fmt.Errorf("list pending skill forge revisions: %w", err)
	}
	modules := s.skillForgeActiveModules()
	moduleItemIDs := make([]string, 0, len(modules)+len(s.gameData.Items)+3)
	for _, module := range modules {
		moduleItemIDs = append(moduleItemIDs, module.ItemID)
	}
	moduleItemIDs = append(moduleItemIDs, s.config.SkillForge.CustomizedNodeSealItemID, s.config.SkillForge.ForgeCatalystItemID, s.config.SkillForge.GoldCostItemID)
	for _, item := range s.gameData.Items {
		if item.ID != "forged_skillbook" && item.HasType("skillbook") && !item.Disabled && strings.TrimSpace(item.UseGrantSkillID) != "" {
			if def, ok := s.skillForgeOfficialDefinition(item.UseGrantSkillID); ok && s.skillForgeOfficialSourceSupported(def) {
				moduleItemIDs = append(moduleItemIDs, item.ID)
			}
		}
	}
	quantityByItem := make(map[string]int64, len(moduleItemIDs))
	rows, err := s.db.Query(ctx, `
		SELECT template_item_id,COALESCE(SUM(quantity),0) FROM player_item_instances
		WHERE user_id=$1 AND template_item_id=ANY($2) AND state='inventory'
		GROUP BY template_item_id
	`, userID, moduleItemIDs)
	if err != nil {
		return SkillForgeState{}, fmt.Errorf("load skill forge inventory: %w", err)
	}
	for rows.Next() {
		var itemID string
		var quantity int64
		if err := rows.Scan(&itemID, &quantity); err != nil {
			rows.Close()
			return SkillForgeState{}, err
		}
		quantityByItem[itemID] = quantity
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SkillForgeState{}, err
	}
	rows.Close()
	inventory := make([]SkillForgeInventoryEntry, 0, len(moduleItemIDs))
	inventorySeen := make(map[string]struct{}, len(moduleItemIDs))
	for _, itemID := range moduleItemIDs {
		if itemID == "" {
			continue
		}
		if _, exists := inventorySeen[itemID]; exists {
			continue
		}
		inventorySeen[itemID] = struct{}{}
		inventory = append(inventory, SkillForgeInventoryEntry{ItemID: itemID, Quantity: quantityByItem[itemID]})
	}
	sourceBooks := make([]SkillForgeSourceBook, 0)
	ownedIDs := make([]string, 0)
	for _, item := range s.gameData.Items {
		def, ok := s.skillForgeOfficialDefinition(item.UseGrantSkillID)
		if item.ID == "forged_skillbook" || !item.HasType("skillbook") || item.Disabled || strings.TrimSpace(item.UseGrantSkillID) == "" || !ok || !s.skillForgeOfficialSourceSupported(def) {
			continue
		}
		qty := quantityByItem[item.ID]
		if qty <= 0 {
			continue
		}
		sourceBooks = append(sourceBooks, SkillForgeSourceBook{ItemID: item.ID, SkillID: item.UseGrantSkillID, Rarity: item.Rarity, IconID: item.IconID, Quantity: qty})
		ownedIDs = append(ownedIDs, item.UseGrantSkillID)
	}
	sort.Slice(sourceBooks, func(i, j int) bool { return sourceBooks[i].ItemID < sourceBooks[j].ItemID })
	sort.Strings(ownedIDs)
	return SkillForgeState{Modules: modules, Limits: s.config.SkillForge, Derived: derived, Inventory: inventory, Drafts: drafts, Skills: skills, PendingRevisions: pending, OwnedSourceIDs: ownedIDs, SourceSkillbooks: sourceBooks}, nil
}

func (s *Store) normalizeLegacySkillForgeBookCounts(ctx context.Context, userID int64) error {
	settings := s.config.SkillForge
	if settings.SourceSkillbookMinCount <= 0 || settings.SourceSkillbookMaxCount < settings.SourceSkillbookMinCount || settings.SourceSkillbookCountStep <= 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		UPDATE player_custom_skill_drafts
		SET source_skillbook_count=$2,revision_no=revision_no+1,updated_at=now()
		WHERE user_id=$1 AND source_kind='official' AND status='active'
		  AND (
		    source_skillbook_count < $2 OR source_skillbook_count > $3 OR
		    MOD(source_skillbook_count - $2, $4) <> 0
		  )
	`, userID, settings.SourceSkillbookMinCount, settings.SourceSkillbookMaxCount, settings.SourceSkillbookCountStep)
	return err
}

func (s *Store) skillForgeActiveModules() []catalog.SkillAlchemyModule {
	out := make([]catalog.SkillAlchemyModule, 0, len(s.gameData.SkillAlchemyModules))
	for _, module := range s.gameData.SkillAlchemyModules {
		if !module.Disabled {
			out = append(out, module)
		}
	}
	return out
}

func (s *Store) CreateSkillForgeDraft(ctx context.Context, userID int64, input CreateSkillForgeDraftInput) (SkillForgeDraft, error) {
	input.SourceKind = strings.ToLower(strings.TrimSpace(input.SourceKind))
	input.SourceSkillbookItemID = strings.TrimSpace(input.SourceSkillbookItemID)
	if input.SourceKind != "blank" && input.SourceKind != "official" {
		return SkillForgeDraft{}, fmt.Errorf("%w: invalid draft source", ErrSkillForgeInvalidGraph)
	}
	switch input.SourceKind {
	case "blank":
		if input.SourceSkillbookItemID != "" || input.SourceSkillbookCount != 0 {
			return SkillForgeDraft{}, fmt.Errorf("%w: blank draft cannot declare a source skill", ErrSkillForgeInvalidGraph)
		}
	case "official":
		if input.SourceSkillbookItemID == "" || !validSkillForgeBookCount(input.SourceSkillbookCount, s.config.SkillForge) {
			return SkillForgeDraft{}, ErrSkillForgeBookCount
		}
	}
	var out SkillForgeDraft
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM player_custom_skill_drafts WHERE user_id=$1 AND status='active'`, userID).Scan(&count); err != nil {
			return err
		}
		if count >= s.config.SkillForge.MaxDrafts {
			return ErrSkillForgeLimit
		}
		base, sourceSkillID, sourceHash, err := s.skillForgeSourceFromBookTx(ctx, tx, input.SourceKind, input.SourceSkillbookItemID)
		if err != nil {
			return err
		}
		if input.Graph.Version == 0 {
			input.Graph.Version = SkillForgeGraphVersion
		}
		input.Graph.Source.Kind = input.SourceKind
		input.Graph.Source.SkillID = sourceSkillID
		input.Graph.Source.DefinitionHash = sourceHash
		if err := validateSkillForgeDraftPayload(input.Graph, s.config.SkillForge); err != nil {
			return err
		}
		if input.SourceKind == "official" {
			if _, _, err := validateSkillForgeGraph(input.Graph, s.skillForgeActiveModules(), s.config.SkillForge); err != nil {
				return err
			}
			_ = base
		}
		graphRaw, err := json.Marshal(input.Graph)
		if err != nil {
			return err
		}
		id, err := newSkillForgeID("draft")
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO player_custom_skill_drafts(id,user_id,source_kind,source_skill_id,source_skillbook_item_id,source_skillbook_count,status,source_definition_hash,graph_jsonb,graph_version,revision_no)
			VALUES($1,$2,$3,$4,$5,$6,'active',$7,$8::jsonb,1,1)
		`, id, userID, input.SourceKind, sourceSkillID, input.SourceSkillbookItemID, input.SourceSkillbookCount, sourceHash, graphRaw)
		if err != nil {
			return fmt.Errorf("insert skill forge draft: %w", err)
		}
		out = SkillForgeDraft{ID: id, SourceKind: input.SourceKind, SourceSkillID: sourceSkillID, SourceSkillbookItemID: input.SourceSkillbookItemID, SourceSkillbookCount: input.SourceSkillbookCount, Status: "active", SourceDefinitionHash: sourceHash, Graph: input.Graph, RevisionNo: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		return nil
	})
	return out, err
}

func (s *Store) UpdateSkillForgeDraft(ctx context.Context, userID int64, draftID string, expectedRevision int64, sourceSkillbookCount int64, graph SkillForgeGraphV1) (SkillForgeDraft, error) {
	var out SkillForgeDraft
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		draft, err := s.skillForgeDraftTx(ctx, tx, userID, draftID, true)
		if err != nil {
			return err
		}
		if draft.RevisionNo != expectedRevision {
			return ErrSkillForgeConflict
		}
		if draft.PendingRevisionID != "" {
			return ErrSkillForgePending
		}
		if draft.Status != "active" {
			return ErrSkillForgeConflict
		}
		if draft.SourceKind == "official" {
			if sourceSkillbookCount == 0 {
				sourceSkillbookCount = draft.SourceSkillbookCount
			}
			if !validSkillForgeBookCount(sourceSkillbookCount, s.config.SkillForge) {
				return ErrSkillForgeBookCount
			}
		} else {
			sourceSkillbookCount = 0
		}
		base, sourceSkillID, sourceHash, err := s.skillForgeSourceFromBookTx(ctx, tx, draft.SourceKind, draft.SourceSkillbookItemID)
		if err != nil {
			return err
		}
		graph.Source = SkillForgeGraphSource{Kind: draft.SourceKind, SkillID: sourceSkillID, DefinitionHash: draft.SourceDefinitionHash}
		if sourceHash != draft.SourceDefinitionHash {
			return ErrSkillForgeSourceStale
		}
		if err := validateSkillForgeDraftPayload(graph, s.config.SkillForge); err != nil {
			return err
		}
		if draft.SourceKind == "official" {
			if _, _, err := validateSkillForgeGraph(graph, s.skillForgeActiveModules(), s.config.SkillForge); err != nil {
				return err
			}
			_ = base
		}
		raw, err := json.Marshal(graph)
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `
			UPDATE player_custom_skill_drafts SET graph_jsonb=$4::jsonb,source_skillbook_count=$5,revision_no=revision_no+1,updated_at=now()
			WHERE id=$1 AND user_id=$2 AND revision_no=$3
			RETURNING revision_no,updated_at
		`, draftID, userID, expectedRevision, raw, sourceSkillbookCount)
		if err := row.Scan(&draft.RevisionNo, &draft.UpdatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSkillForgeConflict
			}
			return err
		}
		draft.Graph = graph
		draft.SourceSkillbookCount = sourceSkillbookCount
		out = draft
		return nil
	})
	return out, err
}

func (s *Store) DeleteSkillForgeDraft(ctx context.Context, userID int64, draftID string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		command, err := tx.Exec(ctx, `DELETE FROM player_custom_skill_drafts WHERE id=$1 AND user_id=$2 AND pending_revision_id IS NULL`, draftID, userID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return ErrSkillForgeNotFound
		}
		return nil
	})
}

func (s *Store) RebaseSkillForgeDraft(ctx context.Context, userID int64, draftID string) (SkillForgeDraft, error) {
	var out SkillForgeDraft
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		draft, err := s.skillForgeDraftTx(ctx, tx, userID, draftID, true)
		if err != nil {
			return err
		}
		if draft.PendingRevisionID != "" {
			return ErrSkillForgePending
		}
		_, _, sourceHash, err := s.skillForgeSourceFromBookTx(ctx, tx, draft.SourceKind, draft.SourceSkillbookItemID)
		if err != nil {
			return err
		}
		draft.SourceDefinitionHash = sourceHash
		draft.Graph.Source.DefinitionHash = sourceHash
		raw, err := json.Marshal(draft.Graph)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `
			UPDATE player_custom_skill_drafts SET source_definition_hash=$3,graph_jsonb=$4::jsonb,revision_no=revision_no+1,updated_at=now()
			WHERE id=$1 AND user_id=$2 RETURNING revision_no,updated_at
		`, draft.ID, userID, sourceHash, raw).Scan(&draft.RevisionNo, &draft.UpdatedAt); err != nil {
			return err
		}
		out = draft
		return nil
	})
	return out, err
}

func (s *Store) PreviewSkillForgeDraft(ctx context.Context, userID int64, draftID string) (SkillForgeCompileResult, error) {
	var out SkillForgeCompileResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		draft, err := s.skillForgeDraftTx(ctx, tx, userID, draftID, false)
		if err != nil {
			return err
		}
		base, _, sourceHash, err := s.skillForgeSourceFromBookTx(ctx, tx, draft.SourceKind, draft.SourceSkillbookItemID)
		if err != nil {
			return err
		}
		if sourceHash != draft.SourceDefinitionHash {
			return ErrSkillForgeSourceStale
		}
		derived, err := s.skillForgeDerivedTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		out, err = CompileSkillForgeGraph(draft.Graph, base, s.skillForgeActiveModules(), s.config.SkillForge, derived, nil, skillForgePreview)
		if err == nil {
			applySkillbookChance(&out, draft, s.config.SkillForge)
		}
		return err
	})
	return out, err
}

func (s *Store) ForgeSkillDraft(ctx context.Context, userID int64, draftID string, expectedRevision int64, expectedGraphHash, idempotencyKey string) (SkillForgeRevision, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 160 {
		return SkillForgeRevision{}, ErrDuplicateIdempotency
	}
	rulesetVersion := s.config.SkillForge.RulesetVersion
	var out SkillForgeRevision
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if existing, found, err := s.skillForgeRevisionByIdempotencyTx(ctx, tx, userID, idempotencyKey); err != nil {
			return err
		} else if found {
			out = existing
			return nil
		}
		if !s.config.SkillForge.RulesetEnabled(rulesetVersion) {
			return ErrSkillForgeRulesetDisabled
		}
		draft, err := s.skillForgeDraftTx(ctx, tx, userID, draftID, true)
		if err != nil {
			return err
		}
		if expectedRevision <= 0 || draft.RevisionNo != expectedRevision {
			return ErrSkillForgeConflict
		}
		if draft.SourceKind == "blank" {
			return ErrSkillForgeBlankOnly
		}
		if draft.SourceKind != "official" || draft.Status != "active" || !validSkillForgeBookCount(draft.SourceSkillbookCount, s.config.SkillForge) {
			return ErrSkillForgeBookCount
		}
		if draft.PendingRevisionID != "" {
			return ErrSkillForgePending
		}
		base, sourceSkillID, sourceHash, err := s.skillForgeSourceFromBookTx(ctx, tx, draft.SourceKind, draft.SourceSkillbookItemID)
		if err != nil {
			return err
		}
		if sourceSkillID != draft.SourceSkillID || sourceHash != draft.SourceDefinitionHash {
			return ErrSkillForgeSourceStale
		}
		derived, err := s.skillForgeDerivedTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		preview, err := CompileSkillForgeGraph(draft.Graph, base, s.skillForgeActiveModules(), s.config.SkillForge, derived, nil, skillForgePreview)
		if err != nil {
			return err
		}
		if !preview.Valid {
			return fmt.Errorf("%w: %s", ErrSkillForgeInvalidGraph, strings.Join(preview.BlockingReasons, "; "))
		}
		applySkillbookChance(&preview, draft, s.config.SkillForge)
		if expectedGraphHash == "" || !strings.EqualFold(preview.GraphHash, strings.TrimSpace(expectedGraphHash)) {
			return ErrSkillForgeConflict
		}
		requested, itemIDs, err := skillForgeRequestedRecipe(preview.Recipe, draft.SourceSkillbookItemID, draft.SourceSkillbookCount)
		if err != nil {
			return err
		}
		ownedByItem, err := s.lockConsumableQuantitiesByTemplateIDsTx(ctx, tx, userID, itemIDs)
		if err != nil {
			return err
		}
		for _, itemID := range itemIDs {
			if ownedByItem[itemID] < requested[itemID] {
				return ErrInsufficientItem
			}
		}
		rng, err := newSkillForgeRNG()
		if err != nil {
			return err
		}
		failureRoll := rng.Float64()
		failed := failureRoll < preview.FinalFailChance
		mode := skillForgeRoll
		if failed {
			mode = skillForgeFloor
		}
		compiled, err := CompileSkillForgeGraph(draft.Graph, base, s.skillForgeActiveModules(), s.config.SkillForge, derived, rng, mode)
		if err != nil {
			return err
		}
		applySkillbookChance(&compiled, draft, s.config.SkillForge)
		compiledRequested, _, err := skillForgeRequestedRecipe(compiled.Recipe, draft.SourceSkillbookItemID, draft.SourceSkillbookCount)
		if err != nil {
			return err
		}
		if !skillForgeRecipesEqual(requested, compiledRequested) {
			return fmt.Errorf("%w: rolled recipe differs from authoritative preview", ErrSkillForgeInvalidGraph)
		}
		consumed := make(map[string]int64, len(requested))
		refunded := make(map[string]int64, len(requested))
		for itemID, qty := range requested {
			if failed {
				refunded[itemID] = qty / 2
				consumed[itemID] = qty - refunded[itemID]
			} else {
				consumed[itemID] = qty
			}
		}
		for _, itemID := range itemIDs {
			qty := consumed[itemID]
			if err := s.LedgerConsumeTemplateTx(ctx, tx, userID, itemID, qty, InventoryLedgerMeta{EventType: WarehouseChangeEventWarehouseOp, Category: InventoryAuditCategoryAlchemy, Direction: InventoryAuditDirectionConsume, SubjectType: "skill_forge", SubjectID: draftID, SourceSurface: "inventory", TargetSurface: "skill_forge", Details: map[string]any{"draftId": draftID, "failed": failed, "requested": requested[itemID], "refunded": refunded[itemID]}}); err != nil {
				return err
			}
		}
		customSkillID, err := newSkillForgeID("custom")
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO player_custom_skills(id,user_id,source_kind,source_skill_id,source_definition_hash,archived_at) VALUES($1,$2,'official',$3,$4,CASE WHEN $5 THEN now() ELSE NULL END)`, customSkillID, userID, draft.SourceSkillID, draft.SourceDefinitionHash, failed)
		if err != nil {
			return err
		}
		compiled.CompiledSkill["id"] = customSkillID
		applySkillForgeIdentity(compiled.CompiledSkill, draft.Graph, s.skillForgeActiveModules())
		revisionNo := int64(1)
		revisionID, err := newSkillForgeID("revision")
		if err != nil {
			return err
		}
		graphRaw, _ := json.Marshal(draft.Graph)
		budget := map[string]any{
			"used":                    compiled.BudgetUsed,
			"limit":                   compiled.BudgetLimit,
			"refund":                  compiled.BudgetRefund,
			"multiplier":              compiled.BudgetMultiplier,
			"baseSuccessChance":       compiled.BaseSuccessChance,
			"materialSuccessModifier": compiled.MaterialSuccessModifier,
			"baseFailChance":          compiled.BaseFailChance,
			"skillbookBonus":          compiled.SkillbookBonus,
			"failChance":              compiled.FinalFailChance,
			"failureRoll":             roundSkillForge(failureRoll),
			"goldCost":                compiled.GoldCost,
			"goldBreakdown":           compiled.GoldCostBreakdown,
		}
		compiled.CompiledSkill["customForge"] = map[string]any{
			"sourceSkillId":         draft.SourceSkillID,
			"sourceKind":            "official",
			"sourceSkillbookItemId": draft.SourceSkillbookItemID,
			"rulesetVersion":        rulesetVersion,
			"budget":                budget,
		}
		skillRaw, _ := json.Marshal(compiled.CompiledSkill)
		recipeRaw, _ := json.Marshal(requested)
		requestedRaw, _ := json.Marshal(requested)
		consumedRaw, _ := json.Marshal(consumed)
		refundedRaw, _ := json.Marshal(refunded)
		rollsRaw, _ := json.Marshal(compiled.Ranges)
		budgetRaw, _ := json.Marshal(budget)
		resultStatus := "pending"
		if failed {
			resultStatus = "failed"
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO player_custom_skill_revisions(id,user_id,custom_skill_id,draft_id,revision_no,source_skill_id,source_definition_hash,graph_jsonb,compiled_skill_jsonb,recipe_jsonb,requested_recipe_jsonb,consumed_recipe_jsonb,refunded_recipe_jsonb,source_skillbook_item_id,source_skillbook_count,budget_jsonb,rolls_jsonb,failed,result_status,compiler_version,ruleset_version,idempotency_key,resolved_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10::jsonb,$11::jsonb,$12::jsonb,$13::jsonb,$14,$15,$16::jsonb,$17::jsonb,$18,$19,$20,$21,$22,CASE WHEN $18 THEN now() ELSE NULL END)
		`, revisionID, userID, customSkillID, draftID, revisionNo, draft.SourceSkillID, draft.SourceDefinitionHash, graphRaw, skillRaw, recipeRaw, requestedRaw, consumedRaw, refundedRaw, draft.SourceSkillbookItemID, draft.SourceSkillbookCount, budgetRaw, rollsRaw, failed, resultStatus, compiled.CompilerVersion, rulesetVersion, idempotencyKey)
		if err != nil {
			return err
		}
		if !failed {
			_, err = tx.Exec(ctx, `UPDATE player_custom_skill_drafts SET custom_skill_id=$3,pending_revision_id=$4,updated_at=now() WHERE id=$1 AND user_id=$2`, draftID, userID, customSkillID, revisionID)
			if err != nil {
				return err
			}
		}
		out = SkillForgeRevision{ID: revisionID, CustomSkillID: customSkillID, DraftID: draftID, RevisionNo: revisionNo, SourceSkillID: draft.SourceSkillID, SourceDefinitionHash: draft.SourceDefinitionHash, Graph: draft.Graph, CompiledSkill: compiled.CompiledSkill, Recipe: requested, RequestedRecipe: requested, ConsumedRecipe: consumed, RefundedRecipe: refunded, SourceSkillbookItemID: draft.SourceSkillbookItemID, SourceSkillbookCount: draft.SourceSkillbookCount, Budget: budget, Rolls: compiled.Ranges, Failed: failed, ResultStatus: resultStatus, CompilerVersion: compiled.CompilerVersion, RulesetVersion: rulesetVersion, CreatedAt: time.Now()}
		if failed {
			receiptRaw, err := json.Marshal(skillForgeFailureReceipt(out))
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO player_skill_forge_idempotency_receipts(user_id,idempotency_key,response_jsonb)
				VALUES($1,$2,$3::jsonb)
				ON CONFLICT(user_id,idempotency_key) DO NOTHING
			`, userID, idempotencyKey, receiptRaw); err != nil {
				return err
			}
			if err := s.pruneSkillForgeFailureHistoryTx(ctx, tx, userID); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func (s *Store) ResolveSkillForgeRevision(ctx context.Context, userID int64, revisionID string, accept bool) (SkillForgeRevision, error) {
	var out SkillForgeRevision
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		revision, err := s.skillForgeRevisionTx(ctx, tx, userID, revisionID, true)
		if err != nil {
			return err
		}
		desiredStatus := "discarded"
		if accept {
			desiredStatus = "accepted"
		}
		if revision.ResultStatus == desiredStatus {
			if accept {
				revision.ForgedSkillbook, _ = s.forgedSkillbookByRevisionTx(ctx, tx, userID, revision.ID)
			}
			out = revision
			return nil
		}
		if revision.ResultStatus != "pending" {
			return ErrSkillForgeConflict
		}
		if accept && !s.config.SkillForge.RulesetEnabled(revision.RulesetVersion) {
			return ErrSkillForgeRulesetDisabled
		}
		status := "discarded"
		if accept {
			status = "accepted"
			rows, err := tx.Query(ctx, `
				SELECT r.ruleset_version
				FROM player_forged_skillbooks b
				JOIN player_custom_skill_revisions r ON r.id=b.revision_id
				WHERE b.user_id=$1 AND b.status IN ('inventory','learned_player','learned_kitty')
			`, userID)
			if err != nil {
				return err
			}
			activeAssets := 0
			for rows.Next() {
				var rulesetVersion int
				if err := rows.Scan(&rulesetVersion); err != nil {
					rows.Close()
					return err
				}
				if s.config.SkillForge.RulesetEnabled(rulesetVersion) {
					activeAssets++
				}
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			rows.Close()
			if activeAssets >= s.config.SkillForge.MaxCustomSkills {
				return ErrSkillForgeLimit
			}
			_, err = tx.Exec(ctx, `
				UPDATE player_custom_skills
				SET current_revision_id=$3, source_definition_hash=$4, archived_at=NULL,
				    updated_at=now()
				WHERE id=$1 AND user_id=$2
			`, revision.CustomSkillID, userID, revision.ID, revision.SourceDefinitionHash)
			if err != nil {
				return err
			}
			instanceID, err := s.AddInventoryItemInstanceTx(ctx, tx, userID, "forged_skillbook", 1, map[string]any{}, nil)
			if err != nil {
				return err
			}
			if err := s.ledgerLogTx(ctx, tx, userID, "forged_skillbook", 1, InventoryLedgerMeta{EventType: WarehouseChangeEventSystemToWarehouse, Category: InventoryAuditCategoryAlchemy, Direction: InventoryAuditDirectionGain, SubjectType: "skill_forge_revision", SubjectID: revision.ID, InstanceID: &instanceID, SourceSurface: "skill_forge", TargetSurface: "inventory"}); err != nil {
				return err
			}
			artifactID, err := newSkillForgeID("forged-book")
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO player_forged_skillbooks(id,user_id,instance_id,custom_skill_id,revision_id,status) VALUES($1,$2,$3,$4,$5,'inventory')`, artifactID, userID, instanceID, revision.CustomSkillID, revision.ID)
			if err != nil {
				return err
			}
			nameParams, _ := revision.CompiledSkill["nameParams"].(map[string]any)
			revision.ForgedSkillbook = &ForgedSkillbookSummary{ID: artifactID, InstanceID: instanceID, CustomSkillID: revision.CustomSkillID, RevisionID: revision.ID, SourceSkillID: revision.SourceSkillID, Status: "inventory", RulesetVersion: revision.RulesetVersion, Usable: true, NameParams: nameParams, CompiledSkill: revision.CompiledSkill, Budget: revision.Budget}
		}
		_, err = tx.Exec(ctx, `UPDATE player_custom_skill_revisions SET result_status=$3,resolved_at=now() WHERE id=$1 AND user_id=$2`, revisionID, userID, status)
		if err != nil {
			return err
		}
		draftStatus := "active"
		if accept {
			draftStatus = "completed"
		}
		_, err = tx.Exec(ctx, `UPDATE player_custom_skill_drafts SET pending_revision_id=NULL,custom_skill_id=CASE WHEN $3 THEN custom_skill_id ELSE NULL END,status=$4,updated_at=now() WHERE id=$1 AND user_id=$2`, revision.DraftID, userID, accept, draftStatus)
		if err != nil {
			return err
		}
		if !accept {
			_, err = tx.Exec(ctx, `UPDATE player_custom_skills SET archived_at=now(),updated_at=now() WHERE id=$1 AND user_id=$2 AND current_revision_id IS NULL`, revision.CustomSkillID, userID)
			if err != nil {
				return err
			}
		}
		revision.ResultStatus = status
		now := time.Now()
		revision.ResolvedAt = &now
		out = revision
		return nil
	})
	return out, err
}

func (s *Store) ArchiveCustomSkill(ctx context.Context, userID int64, skillID string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var hasPending bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM player_custom_skill_revisions
				WHERE user_id=$1 AND custom_skill_id=$2 AND result_status='pending'
			)
		`, userID, skillID).Scan(&hasPending); err != nil {
			return err
		}
		if hasPending {
			return ErrSkillForgePending
		}
		var forgedBookStatus string
		err := tx.QueryRow(ctx, `
			SELECT status FROM player_forged_skillbooks
			WHERE user_id=$1 AND custom_skill_id=$2
			FOR UPDATE
		`, userID, skillID).Scan(&forgedBookStatus)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if forgedBookStatus == "inventory" {
			return ErrForgedSkillbookConflict
		}
		command, err := tx.Exec(ctx, `UPDATE player_custom_skills SET archived_at=now(),updated_at=now() WHERE id=$1 AND user_id=$2 AND archived_at IS NULL`, skillID, userID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return ErrSkillForgeNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM player_skills WHERE user_id=$1 AND skill_id=$2`, userID, skillID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM player_kitty_skills ks USING player_kitties k WHERE ks.kitty_id=k.id AND k.user_id=$1 AND ks.skill_id=$2`, userID, skillID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE player_forged_skillbooks SET status='archived' WHERE user_id=$1 AND custom_skill_id=$2 AND status IN ('learned_player','learned_kitty')`, userID, skillID)
		return err
	})
}

func (s *Store) ListAcceptedCustomSkillDefinitions(ctx context.Context, userID int64) (map[string]map[string]any, error) {
	limit := s.config.SkillForge.MaxCustomSkills
	if limit <= 0 {
		limit = 12
	}
	disabledRulesets := make([]int32, 0, len(s.config.SkillForge.DisabledRulesetVersions))
	for _, version := range s.config.SkillForge.DisabledRulesetVersions {
		disabledRulesets = append(disabledRulesets, int32(version))
	}
	rows, err := s.db.Query(ctx, `
		SELECT cs.id,r.compiled_skill_jsonb,r.ruleset_version FROM player_custom_skills cs
		JOIN player_custom_skill_revisions r ON r.id=cs.current_revision_id
		JOIN player_forged_skillbooks b ON b.custom_skill_id=cs.id
		WHERE cs.user_id=$1 AND cs.archived_at IS NULL AND r.result_status='accepted'
		  AND b.status IN ('learned_player','learned_kitty')
		  AND NOT (r.ruleset_version = ANY($2::int[]))
		ORDER BY cs.created_at DESC
		LIMIT $3
	`, userID, disabledRulesets, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]any{}
	for rows.Next() {
		var id string
		var raw []byte
		var rulesetVersion int
		if err := rows.Scan(&id, &raw, &rulesetVersion); err != nil {
			return nil, err
		}
		if !s.config.SkillForge.RulesetEnabled(rulesetVersion) {
			continue
		}
		var def map[string]any
		if err := json.Unmarshal(raw, &def); err != nil {
			return nil, err
		}
		out[id] = def
	}
	return out, rows.Err()
}

func (s *Store) GetAcceptedCustomSkillDefinition(ctx context.Context, userID int64, skillID string) (map[string]any, bool, error) {
	var raw []byte
	var rulesetVersion int
	err := s.db.QueryRow(ctx, `
		SELECT r.compiled_skill_jsonb,r.ruleset_version FROM player_custom_skills cs
		JOIN player_custom_skill_revisions r ON r.id=cs.current_revision_id
		JOIN player_forged_skillbooks b ON b.custom_skill_id=cs.id
		WHERE cs.user_id=$1 AND cs.id=$2 AND cs.archived_at IS NULL AND r.result_status='accepted'
		  AND b.status IN ('learned_player','learned_kitty')
	`, userID, skillID).Scan(&raw, &rulesetVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !s.config.SkillForge.RulesetEnabled(rulesetVersion) {
		return nil, false, nil
	}
	var def map[string]any
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, false, err
	}
	return def, true, nil
}

func (s *Store) ListSkillForgeDrafts(ctx context.Context, userID int64) ([]SkillForgeDraft, error) {
	limit := s.config.SkillForge.MaxDrafts
	if limit <= 0 {
		limit = 4
	}
	rows, err := s.db.Query(ctx, `SELECT id,COALESCE(custom_skill_id,''),source_kind,source_skill_id,source_skillbook_item_id,source_skillbook_count,status,source_definition_hash,graph_jsonb,revision_no,COALESCE(pending_revision_id,''),created_at,updated_at FROM player_custom_skill_drafts WHERE user_id=$1 AND status='active' ORDER BY updated_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SkillForgeDraft{}
	for rows.Next() {
		var row SkillForgeDraft
		var raw []byte
		if err := rows.Scan(&row.ID, &row.CustomSkillID, &row.SourceKind, &row.SourceSkillID, &row.SourceSkillbookItemID, &row.SourceSkillbookCount, &row.Status, &row.SourceDefinitionHash, &raw, &row.RevisionNo, &row.PendingRevisionID, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &row.Graph); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListPlayerCustomSkills(ctx context.Context, userID int64, includeArchived bool) ([]PlayerCustomSkill, error) {
	limit := s.config.SkillForge.MaxCustomSkills
	if limit <= 0 {
		limit = 12
	}
	if includeArchived {
		limit = s.config.SkillForge.RecentHistoryLimit
		if limit <= 0 {
			limit = skillForgeRecentHistoryLimit
		}
	}
	rows, err := s.db.Query(ctx, `
		SELECT cs.id,cs.source_kind,cs.source_skill_id,cs.source_definition_hash,COALESCE(cs.current_revision_id,''),cs.archived_at IS NOT NULL,COALESCE(r.ruleset_version,0),COALESCE(r.compiled_skill_jsonb,'{}'::jsonb),COALESCE(r.budget_jsonb,'{}'::jsonb),cs.created_at,cs.updated_at
		FROM player_custom_skills cs LEFT JOIN player_custom_skill_revisions r ON r.id=cs.current_revision_id
		WHERE cs.user_id=$1
		  AND ($2 OR (
			cs.archived_at IS NULL
			AND cs.current_revision_id IS NOT NULL
			AND EXISTS (
				SELECT 1 FROM player_forged_skillbooks fsb
				WHERE fsb.user_id=cs.user_id
				  AND fsb.custom_skill_id=cs.id
				  AND fsb.status IN ('learned_player','learned_kitty')
			)
		  ))
		ORDER BY cs.updated_at DESC
		LIMIT $3
	`, userID, includeArchived, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayerCustomSkill{}
	for rows.Next() {
		var row PlayerCustomSkill
		var skillRaw, budgetRaw []byte
		if err := rows.Scan(&row.ID, &row.SourceKind, &row.SourceSkillID, &row.SourceDefinitionHash, &row.CurrentRevisionID, &row.Archived, &row.RulesetVersion, &skillRaw, &budgetRaw, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		row.Usable = !row.Archived && row.CurrentRevisionID != "" && s.config.SkillForge.RulesetEnabled(row.RulesetVersion)
		if row.CurrentRevisionID != "" && !s.config.SkillForge.RulesetEnabled(row.RulesetVersion) {
			row.DisabledReason = "ruleset_disabled"
		}
		_ = json.Unmarshal(skillRaw, &row.CompiledSkill)
		_ = json.Unmarshal(budgetRaw, &row.Budget)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) listSkillForgePendingRevisions(ctx context.Context, userID int64) ([]SkillForgeRevision, error) {
	limit := s.config.SkillForge.MaxDrafts * s.config.SkillForge.MaxPendingResultsPerDraft
	if limit <= 0 {
		limit = 4
	}
	rows, err := s.db.Query(ctx, `SELECT id FROM player_custom_skill_revisions WHERE user_id=$1 AND result_status='pending' ORDER BY created_at LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]SkillForgeRevision, 0, len(ids))
	for _, id := range ids {
		row, err := s.skillForgeRevision(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Store) skillForgeDraftTx(ctx context.Context, tx pgx.Tx, userID int64, draftID string, lock bool) (SkillForgeDraft, error) {
	query := `SELECT id,COALESCE(custom_skill_id,''),source_kind,source_skill_id,source_skillbook_item_id,source_skillbook_count,status,source_definition_hash,graph_jsonb,revision_no,COALESCE(pending_revision_id,''),created_at,updated_at FROM player_custom_skill_drafts WHERE id=$1 AND user_id=$2`
	if lock {
		query += ` FOR UPDATE`
	}
	var row SkillForgeDraft
	var raw []byte
	err := tx.QueryRow(ctx, query, draftID, userID).Scan(&row.ID, &row.CustomSkillID, &row.SourceKind, &row.SourceSkillID, &row.SourceSkillbookItemID, &row.SourceSkillbookCount, &row.Status, &row.SourceDefinitionHash, &raw, &row.RevisionNo, &row.PendingRevisionID, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrSkillForgeNotFound
	}
	if err != nil {
		return row, err
	}
	if err := json.Unmarshal(raw, &row.Graph); err != nil {
		return row, err
	}
	return row, nil
}

func (s *Store) skillForgeSourceFromBookTx(ctx context.Context, tx pgx.Tx, sourceKind, sourceBookItemID string) (map[string]any, string, string, error) {
	switch sourceKind {
	case "blank":
		return nil, "", "", nil
	case "official":
		item, ok := s.skillForgeSourceBookDefinition(sourceBookItemID)
		if !ok {
			return nil, "", "", fmt.Errorf("%w: source is not an eligible official skillbook", ErrSkillForgeInvalidGraph)
		}
		raw, ok := s.skillForgeOfficialDefinition(item.UseGrantSkillID)
		if !ok || !s.skillForgeOfficialSourceSupported(raw) {
			return nil, "", "", fmt.Errorf("%w: official source uses unsupported DSL", ErrSkillForgeInvalidGraph)
		}
		hash, err := SkillForgeDefinitionHash(raw)
		return raw, item.UseGrantSkillID, hash, err
	default:
		return nil, "", "", ErrSkillForgeInvalidGraph
	}
}

func (s *Store) skillForgeSourceBookDefinition(itemID string) (catalog.Item, bool) {
	for _, item := range s.gameData.Items {
		if item.ID == itemID && item.ID != "forged_skillbook" && item.HasType("skillbook") && !item.Disabled && strings.TrimSpace(item.UseGrantSkillID) != "" {
			return item, true
		}
	}
	return catalog.Item{}, false
}

func (s *Store) skillForgeOfficialDefinition(skillID string) (map[string]any, bool) {
	for _, raw := range s.gameData.Skills {
		disabled, _ := raw["disabled"].(bool)
		if stringValue(raw["id"]) == strings.TrimSpace(skillID) && !disabled {
			return raw, true
		}
	}
	return nil, false
}

func validSkillForgeBookCount(count int64, settings config.SkillForgeSettings) bool {
	if settings.SourceSkillbookMinCount <= 0 || settings.SourceSkillbookMaxCount < settings.SourceSkillbookMinCount || settings.SourceSkillbookCountStep <= 0 {
		return false
	}
	return count >= settings.SourceSkillbookMinCount && count <= settings.SourceSkillbookMaxCount &&
		(count-settings.SourceSkillbookMinCount)%settings.SourceSkillbookCountStep == 0
}

func skillForgeBookBonus(count int64, settings config.SkillForgeSettings) float64 {
	if !validSkillForgeBookCount(count, settings) {
		return 0
	}
	steps := float64((count - settings.SourceSkillbookMinCount) / settings.SourceSkillbookCountStep)
	return math.Min(settings.SourceSkillbookMaxBonus, steps*settings.SourceSkillbookBonusStep)
}

func applySkillbookChance(result *SkillForgeCompileResult, draft SkillForgeDraft, settings config.SkillForgeSettings) {
	result.BaseSuccessChance = settings.BaseSuccessChance
	result.BaseFailChance = roundSkillForge(clampSkillForge(1-result.BaseSuccessChance-result.MaterialSuccessModifier, 0, 1))
	result.SkillbookBonus = skillForgeBookBonus(draft.SourceSkillbookCount, settings)
	result.FinalFailChance = roundSkillForge(clampSkillForge(result.BaseFailChance-result.SkillbookBonus, 0, 1))
	result.FailChance = result.FinalFailChance
	result.SourceSkillbookItemID = draft.SourceSkillbookItemID
	result.SourceSkillbookCount = draft.SourceSkillbookCount
}

func skillForgeRequestedRecipe(recipe map[string]int64, sourceBookItemID string, sourceBookCount int64) (map[string]int64, []string, error) {
	requested := make(map[string]int64, len(recipe)+1)
	const maxInt64 = int64(^uint64(0) >> 1)
	add := func(itemID string, quantity int64) error {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" || quantity <= 0 {
			return fmt.Errorf("%w: invalid recipe entry", ErrSkillForgeInvalidGraph)
		}
		if requested[itemID] > maxInt64-quantity {
			return fmt.Errorf("%w: recipe quantity overflow", ErrSkillForgeInvalidGraph)
		}
		requested[itemID] += quantity
		return nil
	}
	for itemID, quantity := range recipe {
		if err := add(itemID, quantity); err != nil {
			return nil, nil, err
		}
	}
	if err := add(sourceBookItemID, sourceBookCount); err != nil {
		return nil, nil, err
	}
	itemIDs := make([]string, 0, len(requested))
	for itemID := range requested {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Strings(itemIDs)
	return requested, itemIDs, nil
}

func skillForgeRecipesEqual(left, right map[string]int64) bool {
	if len(left) != len(right) {
		return false
	}
	for itemID, quantity := range left {
		if right[itemID] != quantity {
			return false
		}
	}
	return true
}

func skillForgeFailureReceipt(row SkillForgeRevision) SkillForgeRevision {
	return SkillForgeRevision{
		ID:                    row.ID,
		CustomSkillID:         row.CustomSkillID,
		DraftID:               row.DraftID,
		RevisionNo:            row.RevisionNo,
		SourceSkillID:         row.SourceSkillID,
		SourceDefinitionHash:  row.SourceDefinitionHash,
		Recipe:                row.Recipe,
		RequestedRecipe:       row.RequestedRecipe,
		ConsumedRecipe:        row.ConsumedRecipe,
		RefundedRecipe:        row.RefundedRecipe,
		SourceSkillbookItemID: row.SourceSkillbookItemID,
		SourceSkillbookCount:  row.SourceSkillbookCount,
		Budget:                row.Budget,
		Failed:                true,
		ResultStatus:          "failed",
		CompilerVersion:       row.CompilerVersion,
		RulesetVersion:        row.RulesetVersion,
		CreatedAt:             row.CreatedAt,
		ResolvedAt:            row.ResolvedAt,
	}
}

func (s *Store) pruneSkillForgeFailureHistoryTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	limit := s.config.SkillForge.RecentHistoryLimit
	if limit <= 0 {
		limit = skillForgeRecentHistoryLimit
	}
	if _, err := tx.Exec(ctx, `
		WITH doomed AS (
			SELECT id,user_id,custom_skill_id,COALESCE(draft_id,'') AS draft_id,revision_no,
			       source_skill_id,source_definition_hash,recipe_jsonb,requested_recipe_jsonb,
			       consumed_recipe_jsonb,refunded_recipe_jsonb,source_skillbook_item_id,
			       source_skillbook_count,budget_jsonb,compiler_version,ruleset_version,
			       idempotency_key,created_at,resolved_at
			FROM player_custom_skill_revisions
			WHERE user_id=$1 AND result_status='failed'
			ORDER BY created_at DESC,id DESC
			OFFSET $2
		)
		INSERT INTO player_skill_forge_idempotency_receipts(user_id,idempotency_key,response_jsonb,created_at)
		SELECT user_id,idempotency_key,jsonb_build_object(
			'id',id,
			'customSkillId',custom_skill_id,
			'draftId',draft_id,
			'revisionNo',revision_no,
			'sourceSkillId',source_skill_id,
			'sourceDefinitionHash',source_definition_hash,
			'recipe',recipe_jsonb,
			'requestedRecipe',requested_recipe_jsonb,
			'consumedRecipe',consumed_recipe_jsonb,
			'refundedRecipe',refunded_recipe_jsonb,
			'sourceSkillbookItemId',source_skillbook_item_id,
			'sourceSkillbookCount',source_skillbook_count,
			'budget',budget_jsonb,
			'failed',true,
			'resultStatus','failed',
			'compilerVersion',compiler_version,
			'rulesetVersion',ruleset_version,
			'createdAt',created_at,
			'resolvedAt',resolved_at
		),created_at
		FROM doomed
		ON CONFLICT(user_id,idempotency_key) DO NOTHING
	`, userID, limit); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		DELETE FROM player_custom_skill_revisions
		WHERE user_id=$1 AND result_status='failed' AND id IN (
			SELECT id FROM player_custom_skill_revisions
			WHERE user_id=$1 AND result_status='failed'
			ORDER BY created_at DESC,id DESC
			OFFSET $2
		)
		RETURNING custom_skill_id
	`, userID, limit)
	if err != nil {
		return err
	}
	customSkillIDs := make([]string, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var customSkillID string
		if err := rows.Scan(&customSkillID); err != nil {
			rows.Close()
			return err
		}
		if _, exists := seen[customSkillID]; !exists {
			seen[customSkillID] = struct{}{}
			customSkillIDs = append(customSkillIDs, customSkillID)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(customSkillIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_custom_skills cs
			WHERE cs.user_id=$1 AND cs.id=ANY($2) AND cs.archived_at IS NOT NULL
			  AND cs.current_revision_id IS NULL
			  AND NOT EXISTS (SELECT 1 FROM player_custom_skill_revisions r WHERE r.custom_skill_id=cs.id)
			  AND NOT EXISTS (SELECT 1 FROM player_forged_skillbooks b WHERE b.custom_skill_id=cs.id)
		`, userID, customSkillIDs); err != nil {
			return err
		}
	}
	retentionDays := s.config.SkillForge.IdempotencyRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	_, err = tx.Exec(ctx, `
		DELETE FROM player_skill_forge_idempotency_receipts
		WHERE user_id=$1 AND created_at < now() - ($2::int * interval '1 day')
	`, userID, retentionDays)
	return err
}

func (s *Store) skillForgeRootSourceTx(ctx context.Context, tx pgx.Tx, userID int64, skillID string) (string, error) {
	skillID = strings.TrimSpace(skillID)
	if !strings.HasPrefix(skillID, "custom:") {
		return skillID, nil
	}
	var root string
	err := tx.QueryRow(ctx, `SELECT source_skill_id FROM player_custom_skills WHERE user_id=$1 AND id=$2 AND archived_at IS NULL`, userID, skillID).Scan(&root)
	if errors.Is(err, pgx.ErrNoRows) {
		return skillID, nil
	}
	return strings.TrimSpace(root), err
}

func (s *Store) validateSkillForgeSourceMutexTx(ctx context.Context, tx pgx.Tx, userID int64, skillIDs []string) error {
	seen := map[string]string{}
	for _, skillID := range sanitizeSkillIDs(skillIDs) {
		root, err := s.skillForgeRootSourceTx(ctx, tx, userID, skillID)
		if err != nil {
			return err
		}
		if previous, exists := seen[root]; exists && !strings.EqualFold(previous, skillID) {
			return fmt.Errorf("%w: %s conflicts with %s", ErrSkillForgeSameSource, skillID, previous)
		}
		seen[root] = skillID
	}
	return nil
}

func (s *Store) skillForgeOfficialSourceSupported(raw map[string]any) bool {
	effects, ok := raw["effects"].([]any)
	if !ok || len(effects) == 0 || len(effects) > s.config.SkillForge.MaxEffects {
		return false
	}
	for _, value := range effects {
		effect, ok := value.(map[string]any)
		if !ok {
			return false
		}
		switch stringValue(effect["kind"]) {
		case "damage", "heal", "status":
		default:
			return false
		}
	}
	return true
}

func (s *Store) skillForgeDerived(ctx context.Context, userID int64) (SkillForgeDerived, error) {
	var out SkillForgeDerived
	err := s.withTx(ctx, func(tx pgx.Tx) error { var err error; out, err = s.skillForgeDerivedTx(ctx, tx, userID); return err })
	return out, err
}
func (s *Store) skillForgeDerivedTx(ctx context.Context, tx pgx.Tx, userID int64) (SkillForgeDerived, error) {
	totals, _, err := s.computePlayerAttributeTotalsTx(ctx, tx, userID, nil)
	if err != nil {
		return SkillForgeDerived{}, err
	}
	level := totals["alchemy_level"]
	if level < 1 {
		level = 1
	}
	return SkillForgeDerived{Level: level, Precision: cleanAlchemyFloat(totals["alchemy_precision"]), Stability: cleanAlchemyFloat(totals["alchemy_craft_success"])}, nil
}

func (s *Store) skillForgeRevision(ctx context.Context, userID int64, id string) (SkillForgeRevision, error) {
	var out SkillForgeRevision
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = s.skillForgeRevisionTx(ctx, tx, userID, id, false)
		return err
	})
	return out, err
}
func (s *Store) skillForgeRevisionTx(ctx context.Context, tx pgx.Tx, userID int64, id string, lock bool) (SkillForgeRevision, error) {
	query := `SELECT id,custom_skill_id,COALESCE(draft_id,''),revision_no,source_skill_id,source_definition_hash,graph_jsonb,compiled_skill_jsonb,recipe_jsonb,requested_recipe_jsonb,consumed_recipe_jsonb,refunded_recipe_jsonb,source_skillbook_item_id,source_skillbook_count,budget_jsonb,rolls_jsonb,failed,result_status,compiler_version,ruleset_version,created_at,resolved_at FROM player_custom_skill_revisions WHERE id=$1 AND user_id=$2`
	if lock {
		query += ` FOR UPDATE`
	}
	var row SkillForgeRevision
	var graphRaw, skillRaw, recipeRaw, requestedRaw, consumedRaw, refundedRaw, budgetRaw, rollsRaw []byte
	err := tx.QueryRow(ctx, query, id, userID).Scan(&row.ID, &row.CustomSkillID, &row.DraftID, &row.RevisionNo, &row.SourceSkillID, &row.SourceDefinitionHash, &graphRaw, &skillRaw, &recipeRaw, &requestedRaw, &consumedRaw, &refundedRaw, &row.SourceSkillbookItemID, &row.SourceSkillbookCount, &budgetRaw, &rollsRaw, &row.Failed, &row.ResultStatus, &row.CompilerVersion, &row.RulesetVersion, &row.CreatedAt, &row.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, ErrSkillForgeNotFound
	}
	if err != nil {
		return row, err
	}
	if err := json.Unmarshal(graphRaw, &row.Graph); err != nil {
		return row, err
	}
	if err := json.Unmarshal(skillRaw, &row.CompiledSkill); err != nil {
		return row, err
	}
	_ = json.Unmarshal(recipeRaw, &row.Recipe)
	_ = json.Unmarshal(requestedRaw, &row.RequestedRecipe)
	_ = json.Unmarshal(consumedRaw, &row.ConsumedRecipe)
	_ = json.Unmarshal(refundedRaw, &row.RefundedRecipe)
	_ = json.Unmarshal(budgetRaw, &row.Budget)
	_ = json.Unmarshal(rollsRaw, &row.Rolls)
	return row, nil
}
func (s *Store) skillForgeRevisionByIdempotencyTx(ctx context.Context, tx pgx.Tx, userID int64, key string) (SkillForgeRevision, bool, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM player_custom_skill_revisions WHERE user_id=$1 AND idempotency_key=$2`, userID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		var raw []byte
		err = tx.QueryRow(ctx, `SELECT response_jsonb FROM player_skill_forge_idempotency_receipts WHERE user_id=$1 AND idempotency_key=$2`, userID, key).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return SkillForgeRevision{}, false, nil
		}
		if err != nil {
			return SkillForgeRevision{}, false, err
		}
		var receipt SkillForgeRevision
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return SkillForgeRevision{}, false, err
		}
		return receipt, true, nil
	}
	if err != nil {
		return SkillForgeRevision{}, false, err
	}
	row, err := s.skillForgeRevisionTx(ctx, tx, userID, id, false)
	return row, true, err
}

func (s *Store) forgedSkillbookByRevisionTx(ctx context.Context, tx pgx.Tx, userID int64, revisionID string) (*ForgedSkillbookSummary, error) {
	var row ForgedSkillbookSummary
	var skillRaw, budgetRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT b.id,COALESCE(b.instance_id,0),b.custom_skill_id,b.revision_id,r.source_skill_id,b.status,r.ruleset_version,COALESCE(b.target_kitty_id,0),r.compiled_skill_jsonb,r.budget_jsonb
		FROM player_forged_skillbooks b JOIN player_custom_skill_revisions r ON r.id=b.revision_id
		WHERE b.user_id=$1 AND b.revision_id=$2
	`, userID, revisionID).Scan(&row.ID, &row.InstanceID, &row.CustomSkillID, &row.RevisionID, &row.SourceSkillID, &row.Status, &row.RulesetVersion, &row.TargetKittyID, &skillRaw, &budgetRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(skillRaw, &row.CompiledSkill)
	_ = json.Unmarshal(budgetRaw, &row.Budget)
	row.Usable = row.Status == "inventory" && s.config.SkillForge.RulesetEnabled(row.RulesetVersion)
	return &row, nil
}

func (s *Store) GetForgedSkillbookByInstance(ctx context.Context, userID, instanceID int64) (*ForgedSkillbookSummary, error) {
	var out *ForgedSkillbookSummary
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var revisionID string
		if err := tx.QueryRow(ctx, `SELECT revision_id FROM player_forged_skillbooks WHERE user_id=$1 AND instance_id=$2`, userID, instanceID).Scan(&revisionID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		var err error
		out, err = s.forgedSkillbookByRevisionTx(ctx, tx, userID, revisionID)
		return err
	})
	return out, err
}

func (s *Store) UseForgedSkillbook(ctx context.Context, userID, instanceID int64, targetType string, kittyID int64) (SkillbookUseResult, error) {
	targetType = strings.ToLower(strings.TrimSpace(targetType))
	if targetType == "" {
		targetType = "player"
	}
	if targetType != "player" && targetType != "kitty" {
		return SkillbookUseResult{}, ErrInvalidSkillID
	}
	var result SkillbookUseResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var artifactID, customSkillID, status string
		var storedKittyID int64
		var rulesetVersion int
		var skillArchived bool
		err := tx.QueryRow(ctx, `
			SELECT b.id,b.custom_skill_id,b.status,COALESCE(b.target_kitty_id,0),r.ruleset_version,cs.archived_at IS NOT NULL
			FROM player_forged_skillbooks b
			JOIN player_custom_skill_revisions r ON r.id=b.revision_id
			JOIN player_custom_skills cs ON cs.id=b.custom_skill_id AND cs.user_id=b.user_id
			WHERE b.user_id=$1 AND b.instance_id=$2
			FOR UPDATE OF b
		`, userID, instanceID).Scan(&artifactID, &customSkillID, &status, &storedKittyID, &rulesetVersion, &skillArchived)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSkillForgeNotFound
		}
		if err != nil {
			return err
		}
		desiredStatus := "learned_player"
		if targetType == "kitty" {
			desiredStatus = "learned_kitty"
			if kittyID <= 0 {
				return ErrKittyNotFound
			}
		}
		if skillArchived {
			return ErrForgedSkillbookConflict
		}
		if status != "inventory" {
			if status == desiredStatus && (desiredStatus == "learned_player" || storedKittyID == kittyID) {
				result = SkillbookUseResult{SkillID: customSkillID, GainedSkill: true, Level: 1, Exp: 0}
				return nil
			}
			return ErrForgedSkillbookConflict
		}
		if !s.config.SkillForge.RulesetEnabled(rulesetVersion) {
			return ErrSkillForgeRulesetDisabled
		}
		if targetType == "kitty" {
			if _, err := s.getOwnedKittyStatusTx(ctx, tx, userID, kittyID, true); err != nil {
				return err
			}
		}
		if targetType == "player" {
			_, err = tx.Exec(ctx, `INSERT INTO player_skills(user_id,skill_id,enabled,cast_order,rule_enabled,rule_groups_jsonb,level,exp,updated_at) VALUES($1,$2,FALSE,0,FALSE,'[]'::jsonb,1,0,now())`, userID, customSkillID)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO player_kitty_skills(kitty_id,skill_id,enabled,cast_order,rule_enabled,rule_groups_jsonb,level,exp,updated_at) VALUES($1,$2,FALSE,0,FALSE,'[]'::jsonb,1,0,now())`, kittyID, customSkillID)
		}
		if err != nil {
			return err
		}
		consumeCommand, err := tx.Exec(ctx, `UPDATE player_item_instances SET state='consumed',archived_at=now(),updated_at=now() WHERE instance_id=$1 AND user_id=$2 AND state='inventory'`, instanceID, userID)
		if err != nil {
			return err
		}
		if consumeCommand.RowsAffected() != 1 {
			return ErrForgedSkillbookConflict
		}
		_, err = tx.Exec(ctx, `UPDATE player_forged_skillbooks SET status=$3,target_kitty_id=NULLIF($4,0),learned_at=now() WHERE id=$1 AND user_id=$2`, artifactID, userID, desiredStatus, kittyID)
		if err != nil {
			return err
		}
		result = SkillbookUseResult{SkillID: customSkillID, GainedSkill: true, Level: 1, Exp: 0}
		return nil
	})
	return result, err
}

func (s *Store) DestroyForgedSkillbook(ctx context.Context, userID, instanceID int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var artifactID, skillID, status string
		if err := tx.QueryRow(ctx, `SELECT id,custom_skill_id,status FROM player_forged_skillbooks WHERE user_id=$1 AND instance_id=$2 FOR UPDATE`, userID, instanceID).Scan(&artifactID, &skillID, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSkillForgeNotFound
			}
			return err
		}
		if status == "destroyed" {
			return nil
		}
		if status != "inventory" {
			return ErrForgedSkillbookConflict
		}
		destroyCommand, err := tx.Exec(ctx, `UPDATE player_item_instances SET state='destroyed',archived_at=now(),updated_at=now() WHERE instance_id=$1 AND user_id=$2 AND state='inventory'`, instanceID, userID)
		if err != nil {
			return err
		}
		if destroyCommand.RowsAffected() != 1 {
			return ErrForgedSkillbookConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE player_forged_skillbooks SET status='destroyed',destroyed_at=now() WHERE id=$1`, artifactID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE player_custom_skills SET archived_at=now(),updated_at=now() WHERE id=$1 AND user_id=$2`, skillID, userID)
		return err
	})
}

func applySkillForgeIdentity(skill map[string]any, graph SkillForgeGraphV1, modules []catalog.SkillAlchemyModule) {
	rootSourceSkillID := graph.Source.SkillID
	if graph.Source.Kind == "custom" {
		if current, ok := skill["nameParams"].(map[string]any); ok {
			rootSourceSkillID = stringValue(current["sourceSkillId"])
		}
	}
	moduleByID := map[string]catalog.SkillAlchemyModule{}
	for _, module := range modules {
		moduleByID[module.ID] = module
	}
	var selected catalog.SkillAlchemyModule
	selectedScore := -1.0
	for _, node := range graph.Nodes {
		if node.Kind != "material" {
			continue
		}
		module, ok := moduleByID[node.ModuleID]
		if !ok {
			continue
		}
		score := module.BudgetCreditPerUnit * float64(node.Quantity)
		if score > selectedScore {
			selected = module
			selectedScore = score
		}
	}
	if selectedScore < 0 {
		return
	}
	skill["icon"] = selected.Icon
	effectKey := "skillForge.name.damage"
	if effects, ok := skill["effects"].([]any); ok && len(effects) > 0 {
		if effect, ok := effects[0].(map[string]any); ok {
			switch stringValue(effect["kind"]) {
			case "heal":
				effectKey = "skillForge.name.heal"
			case "status", "resource":
				effectKey = "skillForge.name.blessing"
			}
		}
	}
	skill["nameParams"] = map[string]any{
		"sourceSkillId": rootSourceSkillID,
		"materialKey":   selected.NameParts["coreKey"],
		"effectKey":     effectKey,
		"traitKey":      selected.NameParts["suffixKey"],
	}
}

func newSkillForgeRNG() (*rand.Rand, error) {
	var seedBytes [8]byte
	if _, err := cryptorand.Read(seedBytes[:]); err != nil {
		return nil, fmt.Errorf("skill forge random seed: %w", err)
	}
	return rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(seedBytes[:])))), nil
}
func newSkillForgeID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := cryptorand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate skill forge id: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw[:])
	uuid := encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
	if prefix == "custom" {
		return "custom:" + uuid, nil
	}
	return prefix + ":" + uuid, nil
}

func validateSkillForgeDraftPayload(graph SkillForgeGraphV1, settings config.SkillForgeSettings) error {
	if graph.Version != SkillForgeGraphVersion {
		return fmt.Errorf("%w: unsupported graph version", ErrSkillForgeInvalidGraph)
	}
	raw, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("marshal skill forge draft: %w", err)
	}
	if len(raw) > settings.MaxGraphBytes || len(graph.Nodes) > settings.MaxNodes || len(graph.Edges) > settings.MaxEdges {
		return fmt.Errorf("%w: draft exceeds configured limits", ErrSkillForgeInvalidGraph)
	}
	seen := map[string]struct{}{}
	for _, node := range graph.Nodes {
		if !validSkillForgeGraphID(node.ID) {
			return fmt.Errorf("%w: invalid node id", ErrSkillForgeInvalidGraph)
		}
		if math.IsNaN(node.Position.X) || math.IsNaN(node.Position.Y) || math.IsInf(node.Position.X, 0) || math.IsInf(node.Position.Y, 0) || math.Abs(node.Position.X) > 1_000_000 || math.Abs(node.Position.Y) > 1_000_000 {
			return fmt.Errorf("%w: invalid node position", ErrSkillForgeInvalidGraph)
		}
		if _, exists := seen[node.ID]; exists {
			return fmt.Errorf("%w: duplicate node id", ErrSkillForgeInvalidGraph)
		}
		seen[node.ID] = struct{}{}
		if node.Kind != "source" && node.Kind != "output" && node.Kind != "effect" && node.Kind != "parameter" && node.Kind != "material" {
			return fmt.Errorf("%w: invalid node kind", ErrSkillForgeInvalidGraph)
		}
		if node.Kind == "parameter" && (!skillForgeValidParameterPath(node) || len(node.DSLPath) > 160) {
			return fmt.Errorf("%w: invalid DSL parameter", ErrSkillForgeInvalidGraph)
		}
		if node.TargetNodeID != "" && !validSkillForgeGraphID(node.TargetNodeID) {
			return fmt.Errorf("%w: invalid material target", ErrSkillForgeInvalidGraph)
		}
	}
	for _, edge := range graph.Edges {
		if !validSkillForgeGraphID(edge.ID) || !validSkillForgeGraphID(edge.Source) || !validSkillForgeGraphID(edge.Target) || !validSkillForgeGraphID(edge.SourceHandle) || !validSkillForgeGraphID(edge.TargetHandle) {
			return fmt.Errorf("%w: invalid edge identifier", ErrSkillForgeInvalidGraph)
		}
	}
	return nil
}
