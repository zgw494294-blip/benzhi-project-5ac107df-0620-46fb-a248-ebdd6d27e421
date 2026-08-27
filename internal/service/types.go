package service

import (
	"encoding/json"
	"time"

	"stagecaption/internal/domain"
	"stagecaption/internal/store"
)

type CreateProjectInput struct {
	Title             string  `json:"title"`
	ProductionVersion string  `json:"productionVersion"`
	FrameRate         float64 `json:"frameRate"`
	DurationMillis    int64   `json:"durationMillis"`
	TimeOrigin        string  `json:"timeOrigin"`
	Actor             string  `json:"actor"`
}
type CueInput struct {
	ID               string `json:"id"`
	Scene            string `json:"scene"`
	Speaker          string `json:"speaker"`
	Text             string `json:"text"`
	StartMillis      int64  `json:"startMillis"`
	EndMillis        int64  `json:"endMillis"`
	Position         int    `json:"position"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Actor            string `json:"actor"`
	LeaseToken       string `json:"leaseToken"`
}
type BatchCueInput struct {
	Scene            string               `json:"scene"`
	ExpectedRevision int64                `json:"expectedRevision"`
	Actor            string               `json:"actor"`
	LeaseToken       string               `json:"leaseToken"`
	Paste            string               `json:"paste,omitempty"`
	Rows             []domain.BatchCueRow `json:"rows,omitempty"`
}
type BatchCueResult struct {
	Project domain.CaptionProject `json:"project"`
	Cues    []domain.CaptionCue   `json:"cues"`
	Count   int                   `json:"count"`
}
type LeaseInput struct {
	Scene      string `json:"scene"`
	Actor      string `json:"actor"`
	TTLSeconds int    `json:"ttlSeconds"`
}
type ValidateInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	Actor            string `json:"actor"`
}
type IssueInput struct {
	CueID    string `json:"cueId"`
	Kind     string `json:"kind"`
	Blocking bool   `json:"blocking"`
	Note     string `json:"note"`
}
type RehearsalInput struct {
	ExpectedRevision int64        `json:"expectedRevision"`
	Recorder         string       `json:"recorder"`
	Notes            string       `json:"notes"`
	StartedAt        time.Time    `json:"startedAt"`
	Issues           []IssueInput `json:"issues"`
}
type RemediationInput struct {
	ExpectedRevision int64           `json:"expectedRevision"`
	Actor            string          `json:"actor"`
	Scene            string          `json:"scene"`
	LeaseToken       string          `json:"leaseToken"`
	Cue              CaptionCuePatch `json:"cue"`
	ResolvedIssueIDs []string        `json:"resolvedIssueIds"`
	ResolutionNote   string          `json:"resolutionNote"`
}
type RemediationPatch struct {
	Cue              CaptionCuePatch `json:"cue"`
	ResolvedIssueIDs []string        `json:"resolvedIssueIds"`
	ResolutionNote   string          `json:"resolutionNote"`
}
type BatchRemediationInput struct {
	ExpectedRevision int64              `json:"expectedRevision"`
	Actor            string             `json:"actor"`
	Scene            string             `json:"scene"`
	LeaseToken       string             `json:"leaseToken"`
	Patches          []RemediationPatch `json:"patches"`
}
type RemediationResult struct {
	Project           domain.CaptionProject   `json:"project"`
	TargetedFindings  []domain.QualityFinding `json:"targetedFindings"`
	RemainingIssueIDs []string                `json:"remainingIssueIds"`
}
type CaptionCuePatch struct {
	ID          string `json:"id"`
	Speaker     string `json:"speaker"`
	Text        string `json:"text"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	Position    int    `json:"position"`
}
type ReviewInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	Reviewer         string `json:"reviewer"`
	Decision         string `json:"decision"`
	Note             string `json:"note"`
}

type ProjectQueueFilter struct {
	Keyword string
	Status  string
	Sort    string
}
type ProjectQueueItem struct {
	domain.CaptionProject
	CueCount                     int    `json:"cueCount"`
	BlockingFindingCount         int    `json:"blockingFindingCount"`
	UnresolvedBlockingIssueCount int    `json:"unresolvedBlockingIssueCount"`
	Risk                         string `json:"risk"`
}

type FindingFilter struct{ Severity, Rule, Scene string }
type FindingItem struct {
	domain.QualityFinding
	Scene       string `json:"scene,omitempty"`
	StartMillis int64  `json:"startMillis,omitempty"`
	EndMillis   int64  `json:"endMillis,omitempty"`
	Position    int    `json:"position,omitempty"`
	TextSummary string `json:"textSummary,omitempty"`
	Scope       string `json:"scope"`
	Invalid     bool   `json:"invalid"`
	RangeStart  int64  `json:"rangeStart,omitempty"`
	RangeEnd    int64  `json:"rangeEnd,omitempty"`
}
type FindingSummary struct {
	Blocking int            `json:"blocking"`
	Warning  int            `json:"warning"`
	ByRule   map[string]int `json:"byRule"`
	ByScene  map[string]int `json:"byScene"`
	Matched  int            `json:"matched"`
}
type FindingView struct {
	Items   []FindingItem  `json:"items"`
	Summary FindingSummary `json:"summary"`
	Rules   []string       `json:"rules"`
	Scenes  []string       `json:"scenes"`
}

type IssueFilter struct{ Scene, Kind, Blocking, Status string }
type IssueItem struct {
	domain.RehearsalIssue
	Rehearsal   *domain.Rehearsal `json:"rehearsal,omitempty"`
	Scene       string            `json:"scene,omitempty"`
	StartMillis int64             `json:"startMillis,omitempty"`
	EndMillis   int64             `json:"endMillis,omitempty"`
	Position    int               `json:"position,omitempty"`
	TextSummary string            `json:"textSummary,omitempty"`
	Executable  bool              `json:"executable"`
}
type ReplayWindow struct {
	Scene       string   `json:"scene"`
	StartMillis int64    `json:"startMillis"`
	EndMillis   int64    `json:"endMillis"`
	IssueIDs    []string `json:"issueIds"`
	CueCount    int      `json:"cueCount"`
}
type IssueSummary struct {
	Pending      int `json:"pending"`
	Resolved     int `json:"resolved"`
	Observations int `json:"observations"`
	Matched      int `json:"matched"`
}
type IssueView struct {
	Items         []IssueItem    `json:"items"`
	Summary       IssueSummary   `json:"summary"`
	ReplayWindows []ReplayWindow `json:"replayWindows"`
	Scenes        []string       `json:"scenes"`
}

type RevisionComparison struct {
	FromRevision int64                  `json:"fromRevision"`
	ToRevision   int64                  `json:"toRevision"`
	Differences  []domain.CueDifference `json:"differences"`
}
type GateItem struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}
type LockGate struct {
	Passed       bool       `json:"passed"`
	Items        []GateItem `json:"items"`
	FromRevision int64      `json:"fromRevision"`
	ToRevision   int64      `json:"toRevision"`
}

type Workspace struct {
	Project     domain.CaptionProject   `json:"project"`
	Cues        []domain.CaptionCue     `json:"cues"`
	Findings    []domain.QualityFinding `json:"findings"`
	Rehearsals  []domain.Rehearsal      `json:"rehearsals"`
	Issues      []domain.RehearsalIssue `json:"issues"`
	Audits      []domain.AuditRecord    `json:"audits"`
	Release     *domain.ReleaseBundle   `json:"release,omitempty"`
	Diff        *store.RevisionDiff     `json:"diff,omitempty"`
	FindingView FindingView             `json:"findingView"`
	IssueView   IssueView               `json:"issueView"`
	Comparison  *RevisionComparison     `json:"comparison,omitempty"`
	LockGate    *LockGate               `json:"lockGate,omitempty"`
}

type BundleFiles struct {
	WebVTT     []byte
	Manifest   []byte
	Credential []byte
	Release    domain.ReleaseBundle
}
type Manifest struct {
	Schema            string  `json:"schema"`
	ProjectID         string  `json:"projectId"`
	Title             string  `json:"title"`
	ProductionVersion string  `json:"productionVersion"`
	LockedRevision    int64   `json:"lockedRevision"`
	FrameRate         float64 `json:"frameRate"`
	DurationMillis    int64   `json:"durationMillis"`
	TimeOrigin        string  `json:"timeOrigin"`
	CueCount          int     `json:"cueCount"`
	WebVTTFile        string  `json:"webvttFile"`
	WebVTTDigest      string  `json:"webvttDigest"`
}
type Credential struct {
	Schema           string `json:"schema"`
	ProjectID        string `json:"projectId"`
	LockedRevision   int64  `json:"lockedRevision"`
	WebVTTDigest     string `json:"webvttDigest"`
	ManifestDigest   string `json:"manifestDigest"`
	CredentialDigest string `json:"credentialDigest"`
	Algorithm        string `json:"algorithm"`
}
type VerifyResult struct {
	Valid           bool   `json:"valid"`
	WebVTTValid     bool   `json:"webvttValid"`
	ManifestValid   bool   `json:"manifestValid"`
	CredentialValid bool   `json:"credentialValid"`
	Message         string `json:"message"`
}

type UploadedBundle struct{ WebVTT, Manifest, Credential []byte }
type FileVerification struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
type UploadedBundleVerification struct {
	Valid      bool               `json:"valid"`
	WebVTT     FileVerification   `json:"webvtt"`
	Manifest   FileVerification   `json:"manifest"`
	Credential FileVerification   `json:"credential"`
	Relations  []FileVerification `json:"relations"`
}

func marshalStable(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
