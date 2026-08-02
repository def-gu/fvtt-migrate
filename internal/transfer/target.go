package transfer

import (
	"context"
	"io"

	"github.com/def-gu/fvtt-migrate/internal/content"
)

type Target interface {
	Missing(ctx context.Context, want []content.Digest) ([]content.Digest, error)
	Put(ctx context.Context, d content.Digest, size int64, r io.Reader) error
	Place(ctx context.Context, d content.Digest, rel string) error
	Commit(ctx context.Context) error
}

// A target over the network spends a round trip per call, and an installation
// has tens of thousands of files. Laying them out in batches turns hours of
// latency into seconds.
type BatchPlacer interface {
	PlaceMany(ctx context.Context, entries []content.Placement) error
}
