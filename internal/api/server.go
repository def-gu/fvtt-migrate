package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/content"
)

const maxRequestBytes = 8 << 20

// Declared here rather than imported so that the wire package stays free of the
// transfer package, which speaks it from the other side.
type Target interface {
	Missing(ctx context.Context, want []content.Digest) ([]content.Digest, error)
	Put(ctx context.Context, d content.Digest, size int64, r io.Reader) error
	Place(ctx context.Context, d content.Digest, rel string) error
	Commit(ctx context.Context) error
}

type Server struct {
	Target Target
	Root   string
	Token  string
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc(PathHello, s.guard(s.hello))
	mux.HandleFunc(PathMissing, s.guard(s.missing))
	mux.HandleFunc(PathBlob, s.guard(s.blob))
	mux.HandleFunc(PathPlace, s.guard(s.place))
	mux.HandleFunc(PathPlaceMany, s.guard(s.placeMany))
	mux.HandleFunc(PathCommit, s.guard(s.commit))
	mux.HandleFunc(PathWorlds, s.guard(s.worlds))
}

// Without a token anyone who can reach the port could write files into the
// target, so an empty token refuses every request rather than allowing them.
func (s *Server) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.Token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 {
			fail(w, http.StatusUnauthorized, "auth.token", "wrong or missing token")
			return
		}
		h(w, r)
	}
}

func (s *Server) hello(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method", "GET expected")
		return
	}
	writeJSON(w, Hello{Agent: "fvtt-migrate", Capabilities: Capabilities, Root: s.Root})
}

func (s *Server) missing(w http.ResponseWriter, r *http.Request) {
	var req MissingRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	missing, err := s.Target.Missing(r.Context(), req.Digests)
	if err != nil {
		fail(w, http.StatusBadRequest, "digest.invalid", err.Error())
		return
	}
	if missing == nil {
		missing = []content.Digest{}
	}
	writeJSON(w, MissingResponse{Missing: missing})
}

func (s *Server) blob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		fail(w, http.StatusMethodNotAllowed, "method", "PUT expected")
		return
	}
	digest := content.Digest(strings.TrimPrefix(r.URL.Path, PathBlob))
	size := int64(-1)
	if v := r.Header.Get("Content-Length"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			size = n
		}
	}

	if err := s.Target.Put(r.Context(), digest, size, r.Body); err != nil {
		fail(w, http.StatusBadRequest, "blob.rejected", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) place(w http.ResponseWriter, r *http.Request) {
	var req PlaceRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	if err := s.Target.Place(r.Context(), req.Digest, req.Path); err != nil {
		code := "place.failed"
		if strings.Contains(err.Error(), "unsafe path") {
			code = "path.unsafe"
		}
		fail(w, http.StatusBadRequest, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) placeMany(w http.ResponseWriter, r *http.Request) {
	var req PlaceManyRequest
	if !readJSON(w, r) || !decode(w, r, &req) {
		return
	}
	for _, e := range req.Entries {
		if err := s.Target.Place(r.Context(), e.Digest, e.Path); err != nil {
			code := "place.failed"
			if strings.Contains(err.Error(), "unsafe path") {
				code = "path.unsafe"
			}
			fail(w, http.StatusBadRequest, code, err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) commit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method", "POST expected")
		return
	}
	if err := s.Target.Commit(r.Context()); err != nil {
		fail(w, http.StatusInternalServerError, "commit.failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readJSON(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method", "POST expected")
		return false
	}
	return true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err := dec.Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "body.invalid", err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Failure{Code: code, Message: message})
}
