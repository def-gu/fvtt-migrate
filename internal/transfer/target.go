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
