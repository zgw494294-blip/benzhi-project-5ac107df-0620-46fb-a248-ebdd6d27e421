package web

import (
	"errors"
	"fmt"
	"net/http"
	"stagecaption/internal/domain"
	"stagecaption/internal/service"
	"strconv"
)

func (h *Handler) ValidateProjectHandler(w http.ResponseWriter, r *http.Request) {
	var in service.ValidateInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, f, err := h.Service.Validate(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p, "findings": f})
}
func (h *Handler) BatchRemediateHandler(w http.ResponseWriter, r *http.Request) {
	var in service.BatchRemediationInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.Service.BatchRemediate(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		var conflict *domain.ConflictError
		if errors.As(err, &conflict) || errors.Is(err, domain.ErrLeaseRequired) || errors.Is(err, domain.ErrInvalidState) {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "batch_remediation_failed", "message": err.Error(), "targetedFindings": result.TargetedFindings, "project": result.Project})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) RevisionDiffHandler(w http.ResponseWriter, r *http.Request) {
	from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("起始修订号无效"))
		return
	}
	to, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("结束修订号无效"))
		return
	}
	result, err := h.Service.CompareRevisions(r.Context(), r.PathValue("projectID"), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) LockGateHandler(w http.ResponseWriter, r *http.Request) {
	reviewer := r.URL.Query().Get("reviewer")
	if len([]rune(reviewer)) > 80 {
		writeError(w, fmt.Errorf("复核员不能超过 80 字"))
		return
	}
	result, err := h.Service.CheckLockGate(r.Context(), r.PathValue("projectID"), reviewer)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) RecordRehearsalHandler(w http.ResponseWriter, r *http.Request) {
	var in service.RehearsalInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, issues, err := h.Service.RecordRehearsal(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": p, "issues": issues})
}
func (h *Handler) RemediateHandler(w http.ResponseWriter, r *http.Request) {
	var in service.RemediationInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, findings, err := h.Service.Remediate(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p, "targetedFindings": findings})
}
func (h *Handler) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	var in service.ReviewInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, release, err := h.Service.Review(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p, "release": release})
}
