package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"stagecaption/internal/domain"
)

func digest(data []byte) string { v := sha256.Sum256(data); return hex.EncodeToString(v[:]) }

func buildWebVTT(p domain.CaptionProject, cues []domain.CaptionCue) []byte {
	ordered := append([]domain.CaptionCue(nil), cues...)
	domain.SortCues(ordered)
	var b bytes.Buffer
	b.WriteString("WEBVTT\n\nNOTE StageCaption project ")
	b.WriteString(p.ID)
	b.WriteString(" revision ")
	b.WriteString(fmt.Sprint(p.Revision))
	b.WriteString("\n\n")
	for _, c := range ordered {
		b.WriteString(c.ID)
		b.WriteByte('\n')
		b.WriteString(domain.FormatTimestamp(c.StartMillis))
		b.WriteString(" --> ")
		b.WriteString(domain.FormatTimestamp(c.EndMillis))
		b.WriteByte('\n')
		if c.Speaker != "" {
			b.WriteString("<v ")
			b.WriteString(c.Speaker)
			b.WriteString(">")
		}
		b.WriteString(c.Text)
		b.WriteString("\n\n")
	}
	return b.Bytes()
}

func makeBundle(p domain.CaptionProject, cues []domain.CaptionCue, reviewer, id string, issued domain.ReleaseBundle) (BundleFiles, error) {
	webvtt := buildWebVTT(p, cues)
	webDigest := digest(webvtt)
	manifest := Manifest{Schema: "stagecaption.release-manifest.v1", ProjectID: p.ID, Title: p.Title, ProductionVersion: p.ProductionVersion, LockedRevision: p.Revision, FrameRate: p.FrameRate, DurationMillis: p.DurationMillis, TimeOrigin: p.TimeOrigin, CueCount: len(cues), WebVTTFile: "captions.vtt", WebVTTDigest: webDigest}
	manifestBytes, err := marshalStable(manifest)
	if err != nil {
		return BundleFiles{}, err
	}
	manifestDigest := digest(manifestBytes)
	credentialDigest := digest([]byte(webDigest + "\n" + manifestDigest + "\n" + p.ID + "\n" + fmt.Sprint(p.Revision)))
	cred := Credential{Schema: "stagecaption.release-credential.v1", ProjectID: p.ID, LockedRevision: p.Revision, WebVTTDigest: webDigest, ManifestDigest: manifestDigest, CredentialDigest: credentialDigest, Algorithm: "SHA-256"}
	credentialBytes, err := marshalStable(cred)
	if err != nil {
		return BundleFiles{}, err
	}
	r := issued
	if r.ID == "" {
		r = domain.ReleaseBundle{ID: id, ProjectID: p.ID, LockedRevision: p.Revision, Reviewer: reviewer}
	}
	r.WebVTTDigest = webDigest
	r.ManifestDigest = manifestDigest
	r.CredentialDigest = credentialDigest
	return BundleFiles{WebVTT: webvtt, Manifest: manifestBytes, Credential: credentialBytes, Release: r}, nil
}

func cloneBundleFiles(b BundleFiles) BundleFiles {
	return BundleFiles{
		WebVTT:     append([]byte(nil), b.WebVTT...),
		Manifest:   append([]byte(nil), b.Manifest...),
		Credential: append([]byte(nil), b.Credential...),
		Release:    b.Release,
	}
}

func (s *Service) GetBundle(ctx context.Context, projectID string) (BundleFiles, error) {
	s.bundleMu.RLock()
	cached, ok := s.bundleCache[projectID]
	s.bundleMu.RUnlock()
	if ok {
		return cloneBundleFiles(cached), nil
	}
	r, err := s.Store.GetRelease(ctx, projectID)
	if err != nil {
		return BundleFiles{}, err
	}
	snap, err := s.Store.GetSnapshot(ctx, projectID, r.LockedRevision)
	if err != nil {
		return BundleFiles{}, err
	}
	files, err := makeBundle(snap.Project, snap.Cues, r.Reviewer, r.ID, r)
	if err != nil {
		return files, err
	}
	files.Release.IssuedAt = r.IssuedAt
	if files.Release.WebVTTDigest != r.WebVTTDigest || files.Release.ManifestDigest != r.ManifestDigest || files.Release.CredentialDigest != r.CredentialDigest {
		return files, fmt.Errorf("播出包摘要与锁版记录不一致")
	}
	s.bundleMu.Lock()
	if cached, ok = s.bundleCache[projectID]; ok {
		files = cached
	} else {
		s.bundleCache[projectID] = files
	}
	s.bundleMu.Unlock()
	return cloneBundleFiles(files), nil
}

func VerifyBundle(files BundleFiles) VerifyResult {
	v := VerifyResult{WebVTTValid: digest(files.WebVTT) == files.Release.WebVTTDigest, ManifestValid: digest(files.Manifest) == files.Release.ManifestDigest}
	expected := digest([]byte(files.Release.WebVTTDigest + "\n" + files.Release.ManifestDigest + "\n" + files.Release.ProjectID + "\n" + fmt.Sprint(files.Release.LockedRevision)))
	v.CredentialValid = expected == files.Release.CredentialDigest
	v.Valid = v.WebVTTValid && v.ManifestValid && v.CredentialValid
	if v.Valid {
		v.Message = "播出包完整，三个 SHA-256 摘要均已复算通过"
	} else {
		v.Message = "播出包摘要验证失败"
	}
	return v
}

func parseSingleJSON(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("文件包含多余内容")
	}
	return nil
}

func (s *Service) VerifyUploadedBundle(ctx context.Context, projectID string, uploaded UploadedBundle) (UploadedBundleVerification, error) {
	release, err := s.Store.GetRelease(ctx, projectID)
	if err != nil {
		return UploadedBundleVerification{}, err
	}
	snapshot, err := s.Store.GetSnapshot(ctx, projectID, release.LockedRevision)
	if err != nil {
		return UploadedBundleVerification{}, err
	}
	result := UploadedBundleVerification{WebVTT: FileVerification{Status: "通过", Message: "WebVTT 摘要与锁版记录一致"}, Manifest: FileVerification{Status: "通过", Message: "JSON 清单摘要和元数据一致"}, Credential: FileVerification{Status: "通过", Message: "摘要凭据格式和摘要链一致"}}
	webDigest := digest(uploaded.WebVTT)
	if webDigest != release.WebVTTDigest {
		result.WebVTT = FileVerification{Status: "摘要不符", Message: "WebVTT SHA-256 与锁版记录不一致"}
	}
	var manifest Manifest
	if err := parseSingleJSON(uploaded.Manifest, &manifest); err != nil {
		result.Manifest = FileVerification{Status: "格式错误", Message: "JSON 清单无法解析：" + err.Error()}
	} else if manifest.Schema != "stagecaption.release-manifest.v1" {
		result.Manifest = FileVerification{Status: "格式错误", Message: "JSON 清单 schema 不受支持"}
	} else {
		manifestDigest := digest(uploaded.Manifest)
		if manifestDigest != release.ManifestDigest {
			result.Manifest = FileVerification{Status: "摘要不符", Message: "JSON 清单 SHA-256 与锁版记录不一致"}
		}
		if manifest.ProjectID != projectID || manifest.LockedRevision != release.LockedRevision || manifest.WebVTTFile != "captions.vtt" || manifest.CueCount != len(snapshot.Cues) {
			result.Manifest = FileVerification{Status: "元数据不符", Message: "清单的项目编号、锁定修订、字幕文件名或字幕数量与锁版快照不一致"}
		}
	}
	var credential Credential
	if err := parseSingleJSON(uploaded.Credential, &credential); err != nil {
		result.Credential = FileVerification{Status: "格式错误", Message: "摘要凭据无法解析：" + err.Error()}
	} else if credential.Schema != "stagecaption.release-credential.v1" || credential.Algorithm != "SHA-256" {
		result.Credential = FileVerification{Status: "格式错误", Message: "摘要凭据 schema 或 algorithm 不受支持"}
	} else {
		expectedChain := digest([]byte(credential.WebVTTDigest + "\n" + credential.ManifestDigest + "\n" + credential.ProjectID + "\n" + fmt.Sprint(credential.LockedRevision)))
		if credential.ProjectID != projectID || credential.LockedRevision != release.LockedRevision {
			result.Credential = FileVerification{Status: "元数据不符", Message: "凭据项目编号或锁定修订不一致"}
		} else if credential.CredentialDigest != expectedChain || credential.CredentialDigest != release.CredentialDigest {
			result.Credential = FileVerification{Status: "摘要不符", Message: "凭据摘要链与锁版记录不一致"}
		}
	}
	addRelation := func(ok bool, pass, fail string) {
		status := "通过"
		message := pass
		if !ok {
			status = "摘要不符"
			message = fail
		}
		result.Relations = append(result.Relations, FileVerification{Status: status, Message: message})
	}
	if manifest.Schema != "" {
		addRelation(manifest.WebVTTDigest == webDigest, "清单引用的 WebVTT 摘要一致", "清单引用的 WebVTT 摘要与所选文件不一致")
	}
	if credential.Schema != "" {
		addRelation(credential.WebVTTDigest == webDigest, "凭据引用的 WebVTT 摘要一致", "凭据引用的 WebVTT 摘要与所选文件不一致")
		addRelation(credential.ManifestDigest == digest(uploaded.Manifest), "凭据引用的清单摘要一致", "凭据引用的清单摘要与所选文件不一致")
	}
	result.Valid = result.WebVTT.Status == "通过" && result.Manifest.Status == "通过" && result.Credential.Status == "通过"
	for _, relation := range result.Relations {
		result.Valid = result.Valid && relation.Status == "通过"
	}
	return result, nil
}
