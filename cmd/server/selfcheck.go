package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"stagecaption/internal/domain"
	"stagecaption/internal/service"
)

type selfClient struct {
	base   string
	client *http.Client
}

func (c selfClient) call(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d：%s", method, path, res.StatusCode, string(data))
	}
	if output != nil {
		return json.Unmarshal(data, output)
	}
	return nil
}

func runSelfCheck(ctx context.Context, base string) error {
	c := selfClient{base: base, client: &http.Client{Timeout: 4e9}}
	var health map[string]any
	if err := c.call(ctx, "GET", "/healthz", nil, &health); err != nil {
		return fmt.Errorf("健康检查失败：%w", err)
	}
	var project domain.CaptionProject
	create := service.CreateProjectInput{Title: "自检演出", ProductionVersion: "selfcheck-v1", FrameRate: 25, DurationMillis: 6000, TimeOrigin: "开场信号为 00:00:00.000", Actor: "自检编辑"}
	if err := c.call(ctx, "POST", "/api/projects", create, &project); err != nil {
		return err
	}
	basePath := "/api/projects/" + project.ID
	var lease struct {
		Token string `json:"token"`
	}
	if err := c.call(ctx, "POST", basePath+"/leases", service.LeaseInput{Scene: "第一场", Actor: "自检编辑", TTLSeconds: 120}, &lease); err != nil {
		return err
	}
	cue := service.CueInput{Scene: "第一场", Speaker: "旁白", Text: "欢迎观看", StartMillis: 0, EndMillis: 6000, Position: 1, ExpectedRevision: project.Revision, Actor: "自检编辑", LeaseToken: lease.Token}
	if err := c.call(ctx, "PUT", basePath+"/cues", cue, &project); err != nil {
		return err
	}
	if err := c.call(ctx, "DELETE", basePath+"/leases", map[string]string{"scene": "第一场", "token": lease.Token}, nil); err != nil {
		return err
	}
	var validation struct {
		Project domain.CaptionProject `json:"project"`
	}
	if err := c.call(ctx, "POST", basePath+"/validate", service.ValidateInput{ExpectedRevision: project.Revision, Actor: "自检质检"}, &validation); err != nil {
		return err
	}
	project = validation.Project
	if project.Status != domain.StatusRehearsal {
		return fmt.Errorf("校验后状态异常：%s", project.Status)
	}
	var rehearsal struct {
		Project domain.CaptionProject   `json:"project"`
		Issues  []domain.RehearsalIssue `json:"issues"`
	}
	issue := service.IssueInput{CueID: "", Kind: "语义", Blocking: true, Note: "用词需要按现场口径调整"}
	workspace := service.Workspace{}
	if err := c.call(ctx, "GET", basePath, nil, &workspace); err != nil {
		return err
	}
	issue.CueID = workspace.Cues[0].ID
	if err := c.call(ctx, "POST", basePath+"/rehearsals", service.RehearsalInput{ExpectedRevision: project.Revision, Recorder: "自检记录员", Notes: "完整排演", Issues: []service.IssueInput{issue}}, &rehearsal); err != nil {
		return err
	}
	project = rehearsal.Project
	lease = struct {
		Token string `json:"token"`
	}{}
	if err := c.call(ctx, "POST", basePath+"/leases", service.LeaseInput{Scene: "第一场", Actor: "自检整改编辑"}, &lease); err != nil {
		return err
	}
	remedy := service.RemediationInput{ExpectedRevision: project.Revision, Actor: "自检整改编辑", Scene: "第一场", LeaseToken: lease.Token, Cue: service.CaptionCuePatch{ID: workspace.Cues[0].ID, Speaker: "旁白", Text: "欢迎欣赏", StartMillis: 0, EndMillis: 6000, Position: 1}, ResolvedIssueIDs: []string{rehearsal.Issues[0].ID}, ResolutionNote: "已按现场口径修正"}
	var remediated struct {
		Project domain.CaptionProject `json:"project"`
	}
	if err := c.call(ctx, "POST", basePath+"/remediations", remedy, &remediated); err != nil {
		return err
	}
	project = remediated.Project
	if err := c.call(ctx, "DELETE", basePath+"/leases", map[string]string{"scene": "第一场", "token": lease.Token}, nil); err != nil {
		return err
	}
	if project.Status != domain.StatusReview {
		return fmt.Errorf("整改后状态异常：%s", project.Status)
	}
	var reviewed struct {
		Project domain.CaptionProject `json:"project"`
		Release domain.ReleaseBundle  `json:"release"`
	}
	if err := c.call(ctx, "POST", basePath+"/reviews", service.ReviewInput{ExpectedRevision: project.Revision, Reviewer: "自检独立复核员", Decision: "lock", Note: "证据完整"}, &reviewed); err != nil {
		return err
	}
	if reviewed.Project.Status != domain.StatusLocked {
		return fmt.Errorf("复核后未锁版")
	}
	var verified service.VerifyResult
	if err := c.call(ctx, "POST", basePath+"/bundle/verify", nil, &verified); err != nil {
		return err
	}
	if !verified.Valid {
		return fmt.Errorf("播出包摘要未通过验证")
	}
	var vtt []byte
	res, err := c.client.Get(c.base + basePath + "/bundle/captions.vtt")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	vtt, err = io.ReadAll(res.Body)
	if err != nil || !bytes.HasPrefix(vtt, []byte("WEBVTT")) {
		return fmt.Errorf("WebVTT 下载内容无效")
	}
	return nil
}
