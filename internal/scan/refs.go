package scan

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Ref struct {
	Raw   string
	Path  string
	Where string
}

var mediaExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".avif": true,
	".gif": true, ".svg": true, ".bmp": true, ".tif": true, ".tiff": true, ".ico": true,
	".webm": true, ".mp4": true, ".m4v": true, ".ogv": true, ".mov": true,
	".mp3": true, ".ogg": true, ".oga": true, ".wav": true, ".flac": true,
	".opus": true, ".m4a": true, ".aac": true,
	".glb": true, ".gltf": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
	".json": false,
}

var (
	htmlAttrRe = regexp.MustCompile(`(?i)(?:src|href|data-src)\s*=\s*["']([^"']+)["']`)
	cssURLRe   = regexp.MustCompile(`(?i)url\(\s*["']?([^"')]+)`)
)

// maxScalarLen bounds the strings we inspect. Fog-of-war and some module
// settings store multi-megabyte data URIs that cannot contain a file path.
const maxScalarLen = 1 << 20

// FromDocument reports every asset path referenced by a single stored document.
// External URLs and data URIs are skipped.
func FromDocument(data []byte, sink func(Ref)) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	walk(v, "", sink)
	return nil
}

func walk(v any, where string, sink func(Ref)) {
	switch t := v.(type) {
	case string:
		scanScalar(t, where, sink)
	case map[string]any:
		for k, child := range t {
			walk(child, join(where, k), sink)
		}
	case []any:
		for i, child := range t {
			walk(child, join(where, strconv.Itoa(i)), sink)
		}
	}
}

func scanScalar(s, where string, sink func(Ref)) {
	if len(s) > maxScalarLen {
		return
	}
	if norm, ok := normalize(s); ok {
		sink(Ref{Raw: s, Path: norm, Where: where})
		return
	}
	if !strings.ContainsAny(s, "<(") {
		return
	}
	for _, re := range []*regexp.Regexp{htmlAttrRe, cssURLRe} {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if norm, ok := normalize(m[1]); ok {
				sink(Ref{Raw: m[1], Path: norm, Where: where})
			}
		}
	}
}

// normalize turns a stored reference into a slash-separated path relative to a
// content root, or reports false if it is not a local asset reference.
func normalize(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 2048 {
		return "", false
	}
	if strings.ContainsAny(s, "<>\n\r\t") {
		return "", false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") {
		return "", false
	}
	if strings.Contains(s, "://") || strings.HasPrefix(s, "//") {
		return "", false
	}

	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimPrefix(s, "./")
	if s == "" {
		return "", false
	}
	if !mediaExt[strings.ToLower(path.Ext(s))] {
		return "", false
	}
	return path.Clean(s), true
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
