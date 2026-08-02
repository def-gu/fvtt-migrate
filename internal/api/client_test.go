package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A newer receiving side advertises capabilities this sender has never heard
// of. Refusing them made every addition on one side a break for the other.
func TestHelloIgnoresUnknownCapabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Hello{
			Agent:        "fvtt-migrate",
			Capabilities: []string{CapBase, CapPlaceMany, "transfer/resume", "index/sqlite"},
			Root:         "/srv/data",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	h, err := c.Hello(context.Background())
	if err != nil {
		t.Fatalf("an unknown capability was refused: %v", err)
	}
	if h.Root != "/srv/data" {
		t.Errorf("root = %q", h.Root)
	}
	if !c.BatchesPlacement() {
		t.Error("the advertised batch placement was not picked up")
	}
}

func TestHelloRefusesSomethingThatIsNotAReceiver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Hello{Agent: "fvtt-migrate", Capabilities: []string{"digest/blake3-256"}})
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, "key").Hello(context.Background()); err == nil {
		t.Error("a side without the base capability was accepted")
	}
}

func TestPlacementIsNotBatchedAgainstAnOlderReceiver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, Hello{Agent: "fvtt-migrate", Capabilities: []string{CapBase, "transfer/whole-file"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "key")
	if _, err := c.Hello(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.BatchesPlacement() {
		t.Error("batching was used against a side that never offered it")
	}
}
