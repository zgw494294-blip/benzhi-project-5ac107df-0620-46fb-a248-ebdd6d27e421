package web

import (
	"fmt"
	"net/http"
	"stagecaption/internal/service"
	"strconv"
)

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "StageCaption"})
}
func (h *Handler) ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for key := range query {
		if key != "keyword" && key != "status" && key != "sort" {
			writeError(w, fmt.Errorf("不支持的项目查询参数：%s", key))
			return
		}
	}
	items, err := h.Service.QueryProjects(r.Context(), service.ProjectQueueFilter{Keyword: query.Get("keyword"), Status: query.Get("status"), Sort: query.Get("sort")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": items})
}
func (h *Handler) CreateProjectHandler(w http.ResponseWriter, r *http.Request) {
	var in service.CreateProjectInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	p, err := h.Service.CreateProject(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}
func (h *Handler) GetProjectHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	allowed := map[string]bool{"severity": true, "rule": true, "scene": true, "issueScene": true, "issueKind": true, "issueBlocking": true, "issueStatus": true, "fromRevision": true, "toRevision": true, "reviewer": true}
	for key := range q {
		if !allowed[key] {
			writeError(w, fmt.Errorf("不支持的工作区查询参数：%s", key))
			return
		}
	}
	if len([]rune(q.Get("reviewer"))) > 80 {
		writeError(w, fmt.Errorf("复核员不能超过 80 字"))
		return
	}
	from, to := int64(0), int64(0)
	var err error
	if q.Get("fromRevision") != "" {
		from, err = strconv.ParseInt(q.Get("fromRevision"), 10, 64)
		if err != nil {
			writeError(w, fmt.Errorf("起始修订号无效"))
			return
		}
	}
	if q.Get("toRevision") != "" {
		to, err = strconv.ParseInt(q.Get("toRevision"), 10, 64)
		if err != nil {
			writeError(w, fmt.Errorf("结束修订号无效"))
			return
		}
	}
	workspace, err := h.Service.GetWorkspaceFiltered(r.Context(), r.PathValue("projectID"), service.FindingFilter{Severity: q.Get("severity"), Rule: q.Get("rule"), Scene: q.Get("scene")}, service.IssueFilter{Scene: q.Get("issueScene"), Kind: q.Get("issueKind"), Blocking: q.Get("issueBlocking"), Status: q.Get("issueStatus")}, from, to, q.Get("reviewer"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}
