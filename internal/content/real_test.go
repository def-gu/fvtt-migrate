package content

import (
	"os"
	"testing"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/foundry"
	"github.com/def-gu/fvtt-migrate/internal/scan"
)

func TestRealInstallHashing(t *testing.T) {
	root := os.Getenv("FVTT_ROOT")
	if root == "" {
		t.Skip("FVTT_ROOT not set")
	}

	inst, err := foundry.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := scan.Build(inst.Data, "")
	if err != nil {
		t.Fatal(err)
	}

	var rels []string
	var total int64
	ix.Each(func(rel string, size int64) {
		rels = append(rels, rel)
		total += size
	})

	cache := &Cache{entries: map[string]Entry{}}
	start := time.Now()
	res := HashTree(inst.Data, rels, cache, 0)
	elapsed := time.Since(start)

	mib := float64(total) / (1 << 20)
	t.Logf("files=%d bytes=%.1f MiB elapsed=%s throughput=%.0f MiB/s errors=%d",
		len(rels), mib, elapsed.Round(time.Millisecond), mib/elapsed.Seconds(), len(res.Errors))

	start = time.Now()
	second := HashTree(inst.Data, rels, cache, 0)
	t.Logf("cached pass: elapsed=%s hashed=%d reused=%d",
		time.Since(start).Round(time.Millisecond), second.Hashed, second.Reused)

	unique := map[Digest]int{}
	var dupBytes int64
	for _, e := range res.Entries {
		unique[e.Digest]++
		if unique[e.Digest] > 1 {
			dupBytes += e.Size
		}
	}
	t.Logf("unique blobs=%d of %d files, duplicate bytes=%.1f MiB",
		len(unique), len(res.Entries), float64(dupBytes)/(1<<20))

}
