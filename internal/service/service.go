package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"stagecaption/internal/quality"
	"stagecaption/internal/store"
)

type Service struct {
	Store     *store.Store
	Quality   *quality.Engine
	Now       func() time.Time
	gateMu    sync.RWMutex
	gateCache map[string]LockGate
}

func New(st *store.Store, q *quality.Engine) *Service {
	if q == nil {
		q = quality.New()
	}
	return &Service{Store: st, Quality: q, Now: time.Now, gateCache: make(map[string]LockGate)}
}

func newID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		n := time.Now().UnixNano()
		return prefix + hex.EncodeToString([]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
	}
	return prefix + hex.EncodeToString(b)
}

func cleanActor(actor string) string { return trimLimit(actor, 80) }

// invalidateGateCache drops every cached lock-gate result for the given
// project. Leases are acquired and released without changing the project
// revision, so the gate cache (keyed by revision) must be cleared whenever
// the active lease set changes; otherwise a gate computed when no leases
// were active could be reused to authorize locking while an editor still
// holds a scene lease.
func (s *Service) invalidateGateCache(projectID string) {
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	for key := range s.gateCache {
		if strings.HasPrefix(key, projectID+"\x00") {
			delete(s.gateCache, key)
		}
	}
}

func trimLimit(v string, max int) string {
	r := []rune(v)
	start := 0
	for start < len(r) && (r[start] == ' ' || r[start] == '\t' || r[start] == '\n') {
		start++
	}
	r = r[start:]
	for len(r) > 0 && (r[len(r)-1] == ' ' || r[len(r)-1] == '\t' || r[len(r)-1] == '\n') {
		r = r[:len(r)-1]
	}
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}
