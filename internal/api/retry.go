package api

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"time"
)

const (
	Attempts = 5
	backoff  = 700 * time.Millisecond
)

// A push over the internet meets dropped connections and brief proxy failures.
// Those are worth another try; anything the far side rejected on its merits is
// not, because repeating it produces the same answer.
func Retryable(err error, status int) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if err != nil {
		return true
	}
	return status == http.StatusTooManyRequests ||
		status == http.StatusRequestTimeout ||
		status >= http.StatusInternalServerError
}

func Wait(ctx context.Context, attempt int) error {
	d := backoff << attempt
	d += time.Duration(rand.Int63n(int64(d / 2)))

	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
