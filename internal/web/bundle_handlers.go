package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"stagecaption/internal/service"
)

const maxBundleFileBytes int64 = 2 << 20
const maxBundleUploadBytes int64 = 7 << 20

func (h *Handler) DownloadBundleHandler(w http.ResponseWriter, r *http.Request) {
	files, err := h.Service.GetBundle(r.Context(), r.PathValue("projectID"))
	if err != nil {
		writeError(w, err)
		return
	}
	name := r.PathValue("filename")
	var data []byte
	switch name {
	case "captions.vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		data = files.WebVTT
	case "manifest.json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		data = files.Manifest
	case "credential.json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		data = files.Credential
	default:
		writeError(w, fmt.Errorf("不支持的播出包文件"))
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
func (h *Handler) VerifyBundleHandler(w http.ResponseWriter, r *http.Request) {
	files, err := h.Service.GetBundle(r.Context(), r.PathValue("projectID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, service.VerifyBundle(files))
}

func (h *Handler) VerifyUploadedBundleHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBundleUploadBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, fmt.Errorf("请选择 WebVTT、JSON 清单和摘要凭据文件：%w", err))
		return
	}
	files := service.UploadedBundle{}
	seen := map[string]bool{}
	for {
		part, e := reader.NextPart()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			writeError(w, fmt.Errorf("读取上传文件失败：%w", e))
			return
		}
		name := part.FormName()
		if name != "webvtt" && name != "manifest" && name != "credential" {
			part.Close()
			writeError(w, fmt.Errorf("不支持的上传字段：%s", name))
			return
		}
		if part.FileName() == "" {
			part.Close()
			writeError(w, fmt.Errorf("%s 缺少文件", name))
			return
		}
		expectedName := map[string]string{"webvtt": "captions.vtt", "manifest": "manifest.json", "credential": "credential.json"}[name]
		if part.FileName() != expectedName {
			part.Close()
			writeError(w, fmt.Errorf("%s 文件名应为 %s", name, expectedName))
			return
		}
		if seen[name] {
			part.Close()
			writeError(w, fmt.Errorf("%s 文件重复", name))
			return
		}
		seen[name] = true
		data, e := io.ReadAll(io.LimitReader(part, maxBundleFileBytes+1))
		part.Close()
		if e != nil {
			writeError(w, fmt.Errorf("读取 %s 失败：%w", name, e))
			return
		}
		if int64(len(data)) > maxBundleFileBytes {
			writeError(w, fmt.Errorf("%s 文件超过 2 MiB 限制", name))
			return
		}
		if len(data) == 0 {
			writeError(w, fmt.Errorf("%s 文件为空", name))
			return
		}
		switch name {
		case "webvtt":
			files.WebVTT = data
		case "manifest":
			files.Manifest = data
		case "credential":
			files.Credential = data
		}
	}
	for _, name := range []string{"webvtt", "manifest", "credential"} {
		if !seen[name] {
			writeError(w, fmt.Errorf("缺少 %s 文件", name))
			return
		}
	}
	result, err := h.Service.VerifyUploadedBundle(r.Context(), r.PathValue("projectID"), files)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
