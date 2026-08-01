package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

type ServeOptions struct {
	Root  string
	Token string
}

func NewToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func Handler(target Target, opts ServeOptions) (http.Handler, error) {
	mux := http.NewServeMux()

	srv := &Server{Target: target, Root: opts.Root, Token: opts.Token}
	srv.Routes(mux)

	panel, err := Panel()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", panel)
	return mux, nil
}

// A blob upload has no useful deadline, so only the header read and the idle
// connection are bounded. Leaving those unbounded is what lets an exposed port
// be held open by connections that never send anything.
func Listener(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
