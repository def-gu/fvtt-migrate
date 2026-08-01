package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

func TestPinnedCandidates(t *testing.T) {
	p := foundry.Package{
		Kind:     foundry.KindModule,
		Version:  "1.13.5.1",
		Manifest: "https://github.com/ruipin/fvtt-lib-wrapper/releases/latest/download/module.json",
	}

	got := pinnedCandidates(p)
	if len(got) != 4 {
		t.Fatalf("got %d candidates: %v", len(got), got)
	}
	want := "https://github.com/ruipin/fvtt-lib-wrapper/releases/download/v1.13.5.1/module.json"
	if got[0] != want {
		t.Errorf("first candidate = %q, want %q", got[0], want)
	}
}

func TestPinnedCandidatesSkipsUnknownHosts(t *testing.T) {
	p := foundry.Package{Version: "1.0", Manifest: "https://www.dropbox.com/scl/fi/x/module.json"}
	if got := pinnedCandidates(p); got != nil {
		t.Errorf("got %v, want none", got)
	}
}

func TestPinnedCandidatesUsesSystemManifestName(t *testing.T) {
	p := foundry.Package{
		Kind:     foundry.KindSystem,
		Version:  "7.7.4",
		Manifest: "https://github.com/foundryvtt/pf2e/releases/latest/download/system.json",
	}
	if got := pinnedCandidates(p); !strings.HasSuffix(got[0], "/system.json") {
		t.Errorf("got %q, want a system.json candidate", got[0])
	}
}

func TestFetchRejectsPlainHTTP(t *testing.T) {
	c := New()
	if _, err := c.fetch(context.Background(), foundry.KindModule, "http://example.invalid/module.json"); err == nil {
		t.Error("plain http was accepted")
	}
}

func TestFetchCapsResponseSize(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"x","version":"1.0.0","junk":"`))
		blob := strings.Repeat("A", 4096)
		for i := 0; i < 512; i++ {
			w.Write([]byte(blob))
		}
		w.Write([]byte(`"}`))
	}))
	defer srv.Close()

	c := New()
	c.Client = srv.Client()
	if _, err := c.fetch(context.Background(), foundry.KindModule, srv.URL); err == nil {
		t.Error("oversized manifest was parsed instead of being cut off at the limit")
	}
}

func TestFetchRejectsErrorStatus(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New()
	c.Client = srv.Client()
	if _, err := c.fetch(context.Background(), foundry.KindModule, srv.URL); err == nil {
		t.Error("404 accepted as a manifest")
	}
}
