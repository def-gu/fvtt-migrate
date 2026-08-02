package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/def-gu/fvtt-migrate/internal/api"
	"github.com/def-gu/fvtt-migrate/internal/content"
	"github.com/def-gu/fvtt-migrate/internal/progress"
)

type Progress struct {
	Selected     int   `json:"selected_files"`
	Negotiated   int   `json:"unique_blobs"`
	Uploaded     int   `json:"transferred_blobs"`
	UploadedByte int64 `json:"transferred_bytes"`
	Placed       int   `json:"placed_files"`
	Skipped      int   `json:"already_present_blobs"`
	SkippedByte  int64 `json:"already_present_bytes"`

	WouldSend     int   `json:"would_send_blobs,omitempty"`
	WouldSendByte int64 `json:"would_send_bytes,omitempty"`
}

type Options struct {
	Workers int
	Sink    progress.Sink
	DryRun  bool
}

func Apply(ctx context.Context, sourceRoot string, sel Selection, digests map[string]content.Entry, tgt Target, opts Options) (*Progress, error) {
	if opts.Workers <= 0 {
		opts.Workers = 4
	}

	prog := &Progress{Selected: len(sel.Paths)}

	want := make([]content.Digest, 0, len(sel.Paths))
	byDigest := map[content.Digest]content.Entry{}
	for _, rel := range sel.Paths {
		e, ok := digests[rel]
		if !ok {
			return nil, fmt.Errorf("no digest for %s", rel)
		}
		if _, seen := byDigest[e.Digest]; !seen {
			byDigest[e.Digest] = e
			want = append(want, e.Digest)
		}
	}

	progress.Emit(opts.Sink, progress.Event{Phase: progress.PhaseNegotiating, Total: int64(len(want))})
	missing, err := tgt.Missing(ctx, want)
	if err != nil {
		return nil, err
	}
	needed := make(map[content.Digest]bool, len(missing))
	for _, d := range missing {
		needed[d] = true
	}
	prog.Negotiated = len(want)
	prog.Skipped = len(want) - len(missing)
	for _, d := range want {
		if !needed[d] {
			prog.SkippedByte += byDigest[d].Size
		}
	}

	if opts.DryRun {
		for _, d := range missing {
			prog.WouldSendByte += byDigest[d].Size
		}
		prog.WouldSend = len(missing)
		return prog, nil
	}

	if err := uploadAll(ctx, sourceRoot, missing, byDigest, tgt, opts, prog); err != nil {
		return prog, err
	}

	if err := placeAll(ctx, sel.Paths, digests, tgt, opts, prog); err != nil {
		return prog, err
	}
	return prog, tgt.Commit(ctx)
}

const placeBatch = 2000

func placeAll(ctx context.Context, paths []string, digests map[string]content.Entry, tgt Target, opts Options, prog *Progress) error {
	report := func() {
		progress.Emit(opts.Sink, progress.Event{
			Phase: progress.PhasePlacing,
			Done:  int64(prog.Placed),
			Total: int64(len(paths)),
		})
	}

	batcher, batched := tgt.(BatchPlacer)
	if !batched {
		for _, rel := range paths {
			if err := tgt.Place(ctx, digests[rel].Digest, rel); err != nil {
				return fmt.Errorf("place %s: %w", rel, err)
			}
			prog.Placed++
			report()
		}
		return nil
	}

	for start := 0; start < len(paths); start += placeBatch {
		end := min(start+placeBatch, len(paths))
		entries := make([]content.Placement, 0, end-start)
		for _, rel := range paths[start:end] {
			entries = append(entries, content.Placement{Digest: digests[rel].Digest, Path: rel})
		}
		if err := batcher.PlaceMany(ctx, entries); err != nil {
			return fmt.Errorf("place %d files starting at %s: %w", len(entries), paths[start], err)
		}
		prog.Placed += len(entries)
		report()
	}
	return nil
}

func uploadAll(ctx context.Context, root string, missing []content.Digest, byDigest map[content.Digest]content.Entry, tgt Target, opts Options, prog *Progress) error {
	jobs := make(chan content.Digest)
	errs := make(chan error, opts.Workers)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range jobs {
				e := byDigest[d]
				if err := upload(ctx, root, e, tgt); err != nil {
					select {
					case errs <- err:
					default:
					}
					return
				}
				mu.Lock()
				prog.Uploaded++
				prog.UploadedByte += e.Size
				done, sent := int64(prog.Uploaded), prog.UploadedByte
				mu.Unlock()
				progress.Emit(opts.Sink, progress.Event{
					Phase:  progress.PhaseTransferring,
					Done:   done,
					Total:  int64(len(missing)),
					Bytes:  sent,
					Detail: e.Path,
				})
			}
		}()
	}

	for _, d := range missing {
		select {
		case jobs <- d:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}

func upload(ctx context.Context, root string, e content.Entry, tgt Target) error {
	var last error
	for attempt := 0; attempt < api.Attempts; attempt++ {
		last = putOnce(ctx, root, e, tgt)
		var transient *api.Transient
		if last == nil || !errors.As(last, &transient) {
			return last
		}
		if attempt+1 < api.Attempts {
			if err := api.Wait(ctx, attempt); err != nil {
				return err
			}
		}
	}
	return last
}

func putOnce(ctx context.Context, root string, e content.Entry, tgt Target) error {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(e.Path)))
	if err != nil {
		return err
	}
	defer f.Close()
	return tgt.Put(ctx, e.Digest, e.Size, f)
}
