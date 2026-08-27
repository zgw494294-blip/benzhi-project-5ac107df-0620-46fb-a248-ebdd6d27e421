package web

import (
	"net/http"
	"stagecaption/internal/service"
	"strconv"
)

func (h *Handler) AcquireLeaseHandler(w http.ResponseWriter, r *http.Request) {
	var in service.LeaseInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	token, expires, err := h.Service.AcquireLease(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "expiresAt": expires})
}
func (h *Handler) ReleaseLeaseHandler(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Scene string `json:"scene"`
		Token string `json:"token"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	if err := h.Service.ReleaseLease(r.Context(), r.PathValue("projectID"), in.Scene, in.Token); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"released": true})
}
func (h *Handler) UpsertCueHandler(w http.ResponseWriter, r *http.Request) {
	var in service.CueInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.Service.UpsertCue(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
func (h *Handler) BatchCueHandler(w http.ResponseWriter, r *http.Request) {
	var in service.BatchCueInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	result, err := h.Service.BatchUpsertCues(r.Context(), r.PathValue("projectID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) DeleteCueHandler(w http.ResponseWriter, r *http.Request) {
	expected, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil {
		writeError(w, err)
		return
	}
	p, err := h.Service.DeleteCue(r.Context(), r.PathValue("projectID"), r.PathValue("cueID"), r.URL.Query().Get("scene"), r.URL.Query().Get("lease"), r.URL.Query().Get("actor"), expected)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
