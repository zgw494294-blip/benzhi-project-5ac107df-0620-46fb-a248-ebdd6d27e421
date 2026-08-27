package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"stagecaption/internal/domain"
	"stagecaption/internal/service"
)

const maxBodyBytes = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("请求 JSON 无效：%w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("请求只能包含一个 JSON 对象")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	code := "invalid_request"
	var conflict *domain.ConflictError
	var batch *domain.BatchValidationError
	var gate *service.LockGateError
	switch {
	case errors.As(err, &conflict):
		status = http.StatusConflict
		code = "revision_conflict"
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		code = "not_found"
	case errors.Is(err, domain.ErrLocked):
		status = http.StatusLocked
		code = "project_locked"
	case errors.Is(err, domain.ErrWriteBarrier), errors.Is(err, domain.ErrLeaseRequired):
		status = http.StatusConflict
		code = "lease_conflict"
	case errors.Is(err, domain.ErrInvalidState):
		status = http.StatusConflict
		code = "invalid_state"
	case errors.Is(err, domain.ErrReviewerEditor):
		status = http.StatusConflict
		code = "reviewer_not_independent"
	}
	body := map[string]any{"error": code, "message": err.Error()}
	if conflict != nil {
		body["currentRevision"] = conflict.CurrentRevision
	}
	if errors.As(err, &batch) {
		body["rowErrors"] = batch.Rows
	}
	if errors.As(err, &gate) {
		body["gate"] = gate.Gate
	}
	writeJSON(w, status, body)
}
