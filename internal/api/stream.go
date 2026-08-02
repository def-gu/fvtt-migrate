package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/progress"
)

// One line of JSON per event, flushed as it is written, so a long operation
// shows its phase and its current file while it runs.
type stream struct {
	enc     *json.Encoder
	flusher http.Flusher
	mu      sync.Mutex
}

func newStream(w http.ResponseWriter) *stream {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	f, _ := w.(http.Flusher)
	return &stream{enc: json.NewEncoder(w), flusher: f}
}

func (s *stream) sink() progress.Sink {
	return progress.Throttle(progress.Func(s.write), 100*time.Millisecond)
}

func (s *stream) write(v any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enc.Encode(v)
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *stream) finish(result any, err error) {
	if err != nil {
		s.write(map[string]any{"type": "failed", "message": err.Error()})
		return
	}
	s.write(map[string]any{"type": "result", "result": result})
}
