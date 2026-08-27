package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"stagecaption/internal/quality"
	"stagecaption/internal/service"
	"stagecaption/internal/store"
	"strings"
	"testing"
)

func TestWorkbenchAndAPI(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	server := httptest.NewServer(New(service.New(st, quality.New())))
	defer server.Close()
	res, err := http.Get(server.URL + "/workbench")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("工作台或安全响应头缺失")
	}
	payload := `{"title":"演出","productionVersion":"v1","frameRate":25,"durationMillis":6000,"timeOrigin":"开场","actor":"编辑"}`
	res, err = http.Post(server.URL+"/api/projects", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("建档状态=%d", res.StatusCode)
	}
}
