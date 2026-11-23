package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"mysecondwebgame/server/internal/store"

	"github.com/gofiber/fiber/v2"
)

type skillForgeCreateDraftRequest struct {
	SourceKind            string                  `json:"sourceKind"`
	SourceSkillbookItemID string                  `json:"sourceSkillbookItemId"`
	SourceSkillbookCount  int64                   `json:"sourceSkillbookCount"`
	Graph                 store.SkillForgeGraphV1 `json:"graph"`
}

type skillForgeUpdateDraftRequest struct {
	ExpectedRevision     int64                   `json:"expectedRevision"`
	SourceSkillbookCount int64                   `json:"sourceSkillbookCount"`
	Graph                store.SkillForgeGraphV1 `json:"graph"`
}

type skillForgeForgeRequest struct {
	ExpectedRevision  int64  `json:"expectedRevision"`
	ExpectedGraphHash string `json:"expectedGraphHash"`
}

func (s *Server) handleSkillForgeState(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	state, err := s.deps.Store.SkillForgeState(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SKILL_FORGE_STATE_FAILED", "failed to load skill forge", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, state)
}

func (s *Server) handleSkillForgeCreateDraft(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req skillForgeCreateDraftRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_INVALID_PAYLOAD", "invalid skill forge payload", err.Error())
	}
	draft, err := s.deps.Store.CreateSkillForgeDraft(c.Context(), userID, store.CreateSkillForgeDraftInput{SourceKind: req.SourceKind, SourceSkillbookItemID: req.SourceSkillbookItemID, SourceSkillbookCount: req.SourceSkillbookCount, Graph: req.Graph})
	if err != nil {
		return s.writeSkillForgeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusCreated, draft)
}

func (s *Server) handleSkillForgeUpdateDraft(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	var req skillForgeUpdateDraftRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_INVALID_PAYLOAD", "invalid skill forge payload", err.Error())
	}
	draft, err := s.deps.Store.UpdateSkillForgeDraft(c.Context(), userID, skillForgePathID(c, "draftId"), req.ExpectedRevision, req.SourceSkillbookCount, req.Graph)
	if err != nil {
		return s.writeSkillForgeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, draft)
}

func (s *Server) handleSkillForgeDeleteDraft(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if err := s.deps.Store.DeleteSkillForgeDraft(c.Context(), userID, skillForgePathID(c, "draftId")); err != nil {
		return s.writeSkillForgeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleSkillForgeRebaseDraft(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	draft, err := s.deps.Store.RebaseSkillForgeDraft(c.Context(), userID, skillForgePathID(c, "draftId"))
	if err != nil {
		return s.writeSkillForgeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, draft)
}

func (s *Server) handleSkillForgePreview(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	result, err := s.deps.Store.PreviewSkillForgeDraft(c.Context(), userID, skillForgePathID(c, "draftId"))
	if err != nil {
		if errors.Is(err, store.ErrSkillForgeInvalidGraph) || errors.Is(err, store.ErrSkillForgeBudget) || errors.Is(err, store.ErrSkillForgeLevelTooLow) || errors.Is(err, store.ErrSkillForgeSourceStale) {
			return s.writeAPISuccess(c, fiber.StatusOK, store.SkillForgeCompileResult{Valid: false, BlockingReasons: []string{err.Error()}, CompilerVersion: s.deps.Config.GlobalSettings.SkillForge.CompilerVersion})
		}
		return s.writeSkillForgeError(c, err)
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleSkillForgeForge(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	key := strings.TrimSpace(c.Get("X-Idempotency-Key"))
	if key == "" {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_IDEMPOTENCY_REQUIRED", "missing X-Idempotency-Key", "")
	}
	if !validSkillForgeIdempotencyKey(key) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_IDEMPOTENCY_INVALID", "invalid X-Idempotency-Key", "")
	}
	var req skillForgeForgeRequest
	if err := decodeStrictJSON(c.Body(), &req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_INVALID_PAYLOAD", "invalid skill forge payload", err.Error())
	}
	if req.ExpectedRevision <= 0 || !validSkillForgeGraphHash(req.ExpectedGraphHash) {
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_INVALID_EXPECTATION", "invalid draft revision or graph hash", "")
	}
	result, err := s.deps.Store.ForgeSkillDraft(c.Context(), userID, skillForgePathID(c, "draftId"), req.ExpectedRevision, req.ExpectedGraphHash, key)
	if err != nil {
		return s.writeSkillForgeError(c, err)
	}
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: true})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleSkillForgeAccept(c *fiber.Ctx) error {
	return s.handleSkillForgeResolve(c, true)
}
func (s *Server) handleSkillForgeDiscard(c *fiber.Ctx) error {
	return s.handleSkillForgeResolve(c, false)
}
func (s *Server) handleSkillForgeResolve(c *fiber.Ctx, accept bool) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	result, err := s.deps.Store.ResolveSkillForgeRevision(c.Context(), userID, skillForgePathID(c, "revisionId"), accept)
	if err != nil {
		return s.writeSkillForgeError(c, err)
	}
	s.invalidatePlayerProfileCaches(userID)
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true, Inventory: accept})
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleSkillForgeArchive(c *fiber.Ctx) error {
	userID, err := s.authUserID(c)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "Invalid bearer token", err.Error())
	}
	if err := s.deps.Store.ArchiveCustomSkill(c.Context(), userID, skillForgePathID(c, "skillId")); err != nil {
		return s.writeSkillForgeError(c, err)
	}
	s.invalidatePlayerProfileCaches(userID)
	s.syncAfterMutation(mutationSyncScope{UserIDs: []int64{userID}, PlayerState: true})
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) invalidatePlayerProfileCaches(userID int64) {
	if userID <= 0 {
		return
	}
	s.playerProfileMu.Lock()
	if s.playerProfileEpoch == nil {
		s.playerProfileEpoch = make(map[int64]uint64)
	}
	s.playerProfileEpoch[userID]++
	delete(s.playerProfileMem, userID)
	s.playerProfileMu.Unlock()
	s.playerPublicMu.Lock()
	if s.playerPublicEpoch == nil {
		s.playerPublicEpoch = make(map[int64]uint64)
	}
	s.playerPublicEpoch[userID]++
	delete(s.playerPublicMem, userID)
	s.playerPublicMu.Unlock()
}

func (s *Server) writeSkillForgeError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, store.ErrSkillForgeInvalidGraph), errors.Is(err, store.ErrSkillForgeLevelTooLow):
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_INVALID_GRAPH", "skill graph is invalid", err.Error())
	case errors.Is(err, store.ErrSkillForgeBudget):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_BUDGET_EXCEEDED", "skill budget exceeded", err.Error())
	case errors.Is(err, store.ErrSkillForgeSourceStale):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_SOURCE_STALE", "source skill changed; rebase is required", err.Error())
	case errors.Is(err, store.ErrSkillForgePending):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_PENDING_RESULT", "resolve the pending result first", err.Error())
	case errors.Is(err, store.ErrSkillForgeConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_REVISION_CONFLICT", "draft or result changed", err.Error())
	case errors.Is(err, store.ErrForgedSkillbookConflict):
		return s.writeAPIError(c, fiber.StatusConflict, "FORGED_SKILLBOOK_CONFLICT", "forged skillbook state conflicts with this operation", err.Error())
	case errors.Is(err, store.ErrSkillForgeLimit):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_LIMIT", "skill forge limit reached", err.Error())
	case errors.Is(err, store.ErrSkillForgeRulesetDisabled):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_RULESET_DISABLED", "this skill forge ruleset is disabled", err.Error())
	case errors.Is(err, store.ErrSkillForgeNotFound):
		return s.writeAPIError(c, fiber.StatusNotFound, "SKILL_FORGE_NOT_FOUND", "skill forge record not found", err.Error())
	case errors.Is(err, store.ErrInsufficientItem):
		return s.writeAPIError(c, fiber.StatusConflict, "SKILL_FORGE_INSUFFICIENT_MATERIAL", "insufficient skill forge material", err.Error())
	case errors.Is(err, store.ErrSkillForgeBlankOnly):
		return s.writeAPIError(c, fiber.StatusBadRequest, "BLANK_DRAFT_PREVIEW_ONLY", "blank drafts are preview only", err.Error())
	case errors.Is(err, store.ErrSkillForgeBookCount):
		settings := s.deps.Config.GlobalSettings.SkillForge
		message := fmt.Sprintf("source skillbook count must be %d..%d in steps of %d", settings.SourceSkillbookMinCount, settings.SourceSkillbookMaxCount, settings.SourceSkillbookCountStep)
		return s.writeAPIError(c, fiber.StatusBadRequest, "SKILL_FORGE_INVALID_SKILLBOOK_COUNT", message, err.Error())
	default:
		return s.writeAPIError(c, fiber.StatusInternalServerError, "SKILL_FORGE_FAILED", "skill forge failed", err.Error())
	}
}

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple json values")
		}
		return fmt.Errorf("decode trailing json: %w", err)
	}
	return nil
}

func skillForgePathID(c *fiber.Ctx, name string) string {
	raw := strings.TrimSpace(c.Params(name))
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return strings.TrimSpace(decoded)
}

func validSkillForgeIdempotencyKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == ':' || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func validSkillForgeGraphHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			continue
		}
		return false
	}
	return true
}
