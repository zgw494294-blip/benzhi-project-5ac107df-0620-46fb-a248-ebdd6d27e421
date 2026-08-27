package service

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"stagecaption/internal/quality"
	"stagecaption/internal/store"
)

type Service struct {
	Store       *store.Store
	Quality     *quality.Engine
	Now         func() time.Time
	bundleMu    sync.RWMutex
	bundleCache map[string]BundleFiles
}

func New(st *store.Store, q *quality.Engine) *Service {
	if q == nil {
		q = quality.New()
	}
	return &Service{Store: st, Quality: q, Now: time.Now, bundleCache: make(map[string]BundleFiles)}
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
