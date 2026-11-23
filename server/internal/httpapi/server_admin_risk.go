package httpapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/risk"
)

type adminRiskDispositionRequest struct {
	OperatorName string `json:"operatorName"`
	OperatorNote string `json:"operatorNote"`
}

type adminRiskResolveCaseRequest struct {
	OperatorName string `json:"operatorName"`
	OperatorNote string `json:"operatorNote"`
	Decision     string `json:"decision"`
}

func (s *Server) handleAdminRiskUserDetail(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Params("userID")), 10, 64)
	if err != nil || userID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_USER_ID", "invalid user id", "")
	}
	detail, err := s.deps.Risk.AdminGetUserRiskDetail(c.Context(), userID)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_USER_DETAIL_FAILED", "admin risk user detail failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleAdminRiskClusterDetail(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	clusterID, err := strconv.ParseInt(strings.TrimSpace(c.Params("clusterID")), 10, 64)
	if err != nil || clusterID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_CLUSTER_ID", "invalid cluster id", "")
	}
	detail, err := s.deps.Risk.AdminGetClusterDetail(c.Context(), clusterID)
	if err != nil {
		if errors.Is(err, risk.ErrClusterNotFound) {
			return s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_RISK_CLUSTER_NOT_FOUND", "risk cluster not found", "")
		}
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_CLUSTER_DETAIL_FAILED", "admin risk cluster detail failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, detail)
}

func (s *Server) handleAdminRiskSearch(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	query := strings.TrimSpace(c.Query("q"))
	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	items, err := s.deps.Risk.AdminSearch(c.Context(), query, limit)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_SEARCH_FAILED", "admin risk search failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": items})
}

func (s *Server) handleAdminRiskReviewCases(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	items, err := s.deps.Risk.AdminListReviewCases(c.Context(), 50)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_REVIEW_CASES_FAILED", "admin risk review cases failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"items": items})
}

func (s *Server) handleAdminRiskHighRiskUsers(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	limit := 30
	offset := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_OFFSET", "invalid offset", "")
		}
		offset = parsed
	}
	result, err := s.deps.Risk.AdminListHighRiskUsers(c.Context(), limit, offset)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_HIGH_RISK_USERS_FAILED", "admin risk high risk users failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleAdminRiskClusters(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	limit := 30
	offset := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_OFFSET", "invalid offset", "")
		}
		offset = parsed
	}
	result, err := s.deps.Risk.AdminListClusters(c.Context(), limit, offset)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_CLUSTERS_FAILED", "admin risk clusters failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleAdminRiskClusterRecompute(c *fiber.Ctx) error {
	s.handleAdminRiskDisposition(c, "recompute")
	return nil
}

func (s *Server) handleAdminRiskClusterRestrict(c *fiber.Ctx) error {
	s.handleAdminRiskDisposition(c, "restrict")
	return nil
}

func (s *Server) handleAdminRiskClusterRelease(c *fiber.Ctx) error {
	s.handleAdminRiskDisposition(c, "release")
	return nil
}

func (s *Server) handleAdminRiskClusterBan(c *fiber.Ctx) error {
	s.handleAdminRiskDisposition(c, "ban")
	return nil
}

func (s *Server) handleAdminRiskDisposition(c *fiber.Ctx, action string) {
	if s.deps.Risk == nil {
		_ = s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
		return
	}
	clusterID, err := strconv.ParseInt(strings.TrimSpace(c.Params("clusterID")), 10, 64)
	if err != nil || clusterID <= 0 {
		_ = s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_CLUSTER_ID", "invalid cluster id", "")
		return
	}
	var req adminRiskDispositionRequest
	if err := c.BodyParser(&req); err != nil && len(c.Body()) > 0 {
		_ = s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_PAYLOAD", "invalid request payload", err.Error())
		return
	}
	err = s.deps.Risk.AdminApplyDisposition(c.Context(), risk.DispositionRequest{
		ClusterID:    clusterID,
		Action:       action,
		OperatorName: req.OperatorName,
		OperatorNote: req.OperatorNote,
	})
	if err != nil {
		switch {
		case errors.Is(err, risk.ErrClusterNotFound):
			_ = s.writeAPIError(c, fiber.StatusNotFound, "ADMIN_RISK_CLUSTER_NOT_FOUND", "risk cluster not found", "")
		case errors.Is(err, risk.ErrInvalidDisposition):
			_ = s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_DISPOSITION", "invalid disposition", "")
		default:
			_ = s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_DISPOSITION_FAILED", "admin risk disposition failed", err.Error())
		}
		return
	}
	_ = s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleAdminRiskPasswordGroups(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	limit := 30
	offset := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_OFFSET", "invalid offset", "")
		}
		offset = parsed
	}
	result, err := s.deps.Risk.AdminListPasswordGroups(c.Context(), limit, offset)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_PASSWORD_GROUPS_FAILED", "admin risk password groups failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleAdminRiskResetAll(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	if err := s.deps.Risk.AdminResetAllRiskData(c.Context()); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_RESET_FAILED", "admin risk reset failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}

func (s *Server) handleAdminRiskAuctionTransfers(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	limit := 50
	offset := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_LIMIT", "invalid limit", "")
		}
		limit = parsed
	}
	if rawOffset := strings.TrimSpace(c.Query("offset")); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_OFFSET", "invalid offset", "")
		}
		offset = parsed
	}
	result, err := s.deps.Risk.AdminListAuctionTransfers(c.Context(), limit, offset)
	if err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_AUCTION_TRANSFERS_FAILED", "admin risk auction transfers failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, result)
}

func (s *Server) handleAdminRiskReviewCaseResolve(c *fiber.Ctx) error {
	if s.deps.Risk == nil {
		return s.writeAPIError(c, fiber.StatusServiceUnavailable, "ADMIN_RISK_UNAVAILABLE", "risk service unavailable", "")
	}
	caseID, err := strconv.ParseInt(strings.TrimSpace(c.Params("caseID")), 10, 64)
	if err != nil || caseID <= 0 {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_CASE_ID", "invalid case id", "")
	}
	var req adminRiskResolveCaseRequest
	if err := c.BodyParser(&req); err != nil {
		return s.writeAPIError(c, fiber.StatusBadRequest, "ADMIN_RISK_INVALID_PAYLOAD", "invalid request payload", err.Error())
	}
	if err := s.deps.Risk.AdminResolveReviewCase(c.Context(), caseID, req.OperatorName, req.OperatorNote, req.Decision); err != nil {
		return s.writeAPIError(c, fiber.StatusInternalServerError, "ADMIN_RISK_REVIEW_CASE_RESOLVE_FAILED", "admin risk review case resolve failed", err.Error())
	}
	return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}
