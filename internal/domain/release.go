package domain

import "time"

type ReleaseBundle struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"projectId"`
	LockedRevision   int64     `json:"lockedRevision"`
	WebVTTDigest     string    `json:"webvttDigest"`
	ManifestDigest   string    `json:"manifestDigest"`
	CredentialDigest string    `json:"credentialDigest"`
	Reviewer         string    `json:"reviewer"`
	IssuedAt         time.Time `json:"issuedAt"`
}

type AuditRecord struct {
	ID        int64     `json:"id"`
	ProjectID string    `json:"projectId"`
	Revision  int64     `json:"revision"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"createdAt"`
}

type RevisionSnapshot struct {
	Project   CaptionProject `json:"project"`
	Cues      []CaptionCue   `json:"cues"`
	CreatedAt time.Time      `json:"createdAt"`
}
