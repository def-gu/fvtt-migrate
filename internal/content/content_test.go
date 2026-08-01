package content

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHashFileMatchesKnownVector(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.txt", "abc")

	got, err := HashFile(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85"
	if string(got) != want {
		t.Errorf("blake3(\"abc\") = %s, want %s", got, want)
	}
}

func TestIdenticalContentSharesDigest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "one/map.webp", "same bytes")
	write(t, root, "two/copy.webp", "same bytes")

	res := HashTree(root, []string{"one/map.webp", "two/copy.webp"}, &Cache{entries: map[string]Entry{}}, 2)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %v", res.Errors)
	}
	if res.Entries["one/map.webp"].Digest != res.Entries["two/copy.webp"].Digest {
		t.Error("duplicate files got different digests, deduplication would miss them")
	}
}

func TestCacheAvoidsRehashing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.bin", "payload")
	cache := &Cache{entries: map[string]Entry{}}

	first := HashTree(root, []string{"a.bin"}, cache, 1)
	if first.Hashed != 1 || first.Reused != 0 {
		t.Fatalf("first pass: hashed=%d reused=%d", first.Hashed, first.Reused)
	}

	second := HashTree(root, []string{"a.bin"}, cache, 1)
	if second.Hashed != 0 || second.Reused != 1 {
		t.Errorf("second pass: hashed=%d reused=%d, want the cached digest", second.Hashed, second.Reused)
	}
}

func TestCacheInvalidatesOnChange(t *testing.T) {
	root := t.TempDir()
	p := write(t, root, "a.bin", "before")
	cache := &Cache{entries: map[string]Entry{}}
	HashTree(root, []string{"a.bin"}, cache, 1)

	if err := os.WriteFile(p, []byte("after!"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}

	res := HashTree(root, []string{"a.bin"}, cache, 1)
	if res.Reused != 0 {
		t.Error("stale digest reused after the file changed")
	}
}

func TestMissingFileIsReportedNotFatal(t *testing.T) {
	root := t.TempDir()
	write(t, root, "present.bin", "x")

	res := HashTree(root, []string{"present.bin", "gone.bin"}, &Cache{entries: map[string]Entry{}}, 2)
	if _, ok := res.Entries["present.bin"]; !ok {
		t.Error("present file was skipped")
	}
	if res.Errors["gone.bin"] == nil {
		t.Error("missing file was not reported")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{entries: map[string]Entry{}, path: filepath.Join(dir, "c.json")}
	c.Store(Entry{Path: "a.bin", Size: 3, ModTime: 42, Digest: "deadbeef"})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	reopened := &Cache{entries: load(c.path)}
	if got, ok := reopened.Lookup("a.bin", 3, 42); !ok || got.Digest != "deadbeef" {
		t.Errorf("round trip lost the entry: %+v ok=%v", got, ok)
	}
}
