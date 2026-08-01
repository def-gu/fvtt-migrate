package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/def-gu/fvtt-migrate/internal/content"
)

const negotiateBatch = 4000

type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
}

func NewClient(base, token string) *Client {
	return &Client{
		Base:  strings.TrimSuffix(base, "/"),
		Token: token,
		HTTP:  &http.Client{},
	}
}

// A token sent over plain HTTP to anything but this machine is a token given
// away, so that combination is refused rather than warned about.
func CheckAddress(base string, allowInsecure bool) error {
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("cannot read the address %q", base)
	}
	if u.Scheme == "https" || allowInsecure {
		return nil
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	return fmt.Errorf("%s sends the token in the clear. Use https, or pass --insecure if the network is trusted", base)
}

func (t *Client) Hello(ctx context.Context) (*Hello, error) {
	var h Hello
	if err := t.call(ctx, http.MethodGet, PathHello, nil, &h); err != nil {
		return nil, err
	}

	known := map[string]bool{}
	for _, c := range Capabilities {
		known[c] = true
	}
	var unknown []string
	for _, c := range h.Capabilities {
		if !known[c] {
			unknown = append(unknown, c)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("the receiving side speaks %s, which this version does not. Update fvtt-migrate on this machine",
			strings.Join(unknown, ", "))
	}
	return &h, nil
}

func (t *Client) Missing(ctx context.Context, want []content.Digest) ([]content.Digest, error) {
	var out []content.Digest
	for start := 0; start < len(want); start += negotiateBatch {
		end := min(start+negotiateBatch, len(want))

		var resp MissingResponse
		body := MissingRequest{Digests: want[start:end]}
		if err := t.call(ctx, http.MethodPost, PathMissing, body, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp.Missing...)
	}
	return out, nil
}

func (t *Client) Put(ctx context.Context, d content.Digest, size int64, r io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, t.Base+PathBlob+string(d), r)
	if err != nil {
		return err
	}
	if size >= 0 {
		req.ContentLength = size
	}
	req.Header.Set("Authorization", "Bearer "+t.Token)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return &Transient{Err: err}
	}
	defer resp.Body.Close()
	if Retryable(nil, resp.StatusCode) {
		return &Transient{Err: expect(resp, http.StatusNoContent)}
	}
	return expect(resp, http.StatusNoContent)
}

type Transient struct{ Err error }

func (t *Transient) Error() string { return t.Err.Error() }
func (t *Transient) Unwrap() error { return t.Err }

func (t *Client) Place(ctx context.Context, d content.Digest, rel string) error {
	return t.call(ctx, http.MethodPost, PathPlace, PlaceRequest{Digest: d, Path: rel}, nil)
}

func (t *Client) Commit(ctx context.Context) error {
	return t.call(ctx, http.MethodPost, PathCommit, nil, nil)
}

func (t *Client) call(ctx context.Context, method, path string, body, out any) error {
	var last error
	for attempt := 0; attempt < Attempts; attempt++ {
		last = t.callOnce(ctx, method, path, body, out)
		var transient *Transient
		if !errors.As(last, &transient) {
			return last
		}
		if attempt+1 < Attempts {
			if err := Wait(ctx, attempt); err != nil {
				return err
			}
		}
	}
	return last
}

func (t *Client) callOnce(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, t.Base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+t.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return &Transient{Err: err}
	}
	defer resp.Body.Close()

	if Retryable(nil, resp.StatusCode) {
		return &Transient{Err: expect(resp, http.StatusOK)}
	}

	want := http.StatusOK
	if out == nil {
		want = http.StatusNoContent
	}
	if err := expect(resp, want); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(out)
}

func expect(resp *http.Response, status int) error {
	if resp.StatusCode == status {
		return nil
	}
	var f Failure
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&f) == nil && f.Message != "" {
		return fmt.Errorf("%s (%s)", f.Message, f.Code)
	}
	return fmt.Errorf("unexpected status %s", resp.Status)
}

func (t *Client) Worlds(ctx context.Context) ([]WorldCount, error) {
	var resp WorldsResponse
	if err := t.call(ctx, http.MethodGet, PathWorlds, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Worlds, nil
}
