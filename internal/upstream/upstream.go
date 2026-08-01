package upstream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

const (
	maxManifestBytes = 1 << 20
	defaultTimeout   = 20 * time.Second
	defaultWorkers   = 8
)

type Result struct {
	PinnedManifest  string
	Available       string
	AvailableCompat foundry.Compatibility
	Err             error
}

type Checker struct {
	Client  *http.Client
	Workers int
}

var forgeRepo = regexp.MustCompile(`^https://(github\.com|gitlab\.com)/([^/]+)/([^/]+)/`)

func New() *Checker {
	return &Checker{
		Client: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				return nil
			},
		},
		Workers: defaultWorkers,
	}
}

func (c *Checker) CheckAll(ctx context.Context, pkgs []foundry.Package) map[string]Result {
	out := make(map[string]Result, len(pkgs))
	var mu sync.Mutex

	jobs := make(chan foundry.Package)
	var wg sync.WaitGroup
	for i := 0; i < c.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				r := c.check(ctx, p)
				mu.Lock()
				out[p.ID] = r
				mu.Unlock()
			}
		}()
	}

	for _, p := range pkgs {
		if p.Manifest == "" {
			continue
		}
		select {
		case jobs <- p:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	return out
}

func (c *Checker) check(ctx context.Context, p foundry.Package) Result {
	var r Result

	for _, url := range pinnedCandidates(p) {
		got, err := c.fetch(ctx, p.Kind, url)
		if err != nil || got.Version != p.Version {
			continue
		}
		r.PinnedManifest = url
		break
	}

	latest, err := c.fetch(ctx, p.Kind, p.Manifest)
	if err != nil {
		r.Err = err
		return r
	}
	r.Available = latest.Version
	r.AvailableCompat = latest.Compat
	return r
}

func pinnedCandidates(p foundry.Package) []string {
	m := forgeRepo.FindStringSubmatch(p.Manifest)
	if m == nil || p.Version == "" {
		return nil
	}
	base := "https://" + m[1] + "/" + m[2] + "/" + m[3] + "/releases/download/"
	name := manifestName(p.Kind)

	var out []string
	for _, tag := range []string{"v" + p.Version, p.Version, "release-v" + p.Version, "release-" + p.Version} {
		out = append(out, base+tag+"/"+name)
	}
	return out
}

func manifestName(k foundry.Kind) string {
	if k == foundry.KindSystem {
		return "system.json"
	}
	return "module.json"
}

func (c *Checker) fetch(ctx context.Context, kind foundry.Kind, url string) (*foundry.Package, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, errors.New("refusing non-https manifest URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "fvtt-migrate")
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes))
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		return nil, errors.New("served a web page, not a manifest")
	}
	return foundry.ParseManifest(kind, body)
}
