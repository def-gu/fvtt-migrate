package scan

import (
	"encoding/json"
	"net/url"
	"path"
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

// maxScalarLen bounds the strings we inspect. Fog-of-war and some module
// settings store multi-megabyte data URIs that cannot contain a file path.
const maxScalarLen = 1 << 20

// The field path is assembled only where a reference was found. Documents hold
// millions of fields and thousands of references, so building it on the way
// down cost more than the rest of the scan together.
func FromDocument(data []byte, sink func(Ref)) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	w := walker{sink: sink}
	w.walk(v)
	return nil
}

type walker struct {
	sink func(Ref)
	at   []string
}

func (w *walker) walk(v any) {
	switch t := v.(type) {
	case string:
		w.scanScalar(t)
	case map[string]any:
		for k, child := range t {
			w.at = append(w.at, k)
			w.walk(child)
			w.at = w.at[:len(w.at)-1]
		}
	case []any:
		for i, child := range t {
			w.at = append(w.at, strconv.Itoa(i))
			w.walk(child)
			w.at = w.at[:len(w.at)-1]
		}
	}
}

func (w *walker) scanScalar(s string) {
	if len(s) > maxScalarLen {
		return
	}
	if norm, ok := normalize(s); ok {
		w.sink(Ref{Raw: s, Path: norm, Where: strings.Join(w.at, ".")})
		return
	}
	if !strings.ContainsAny(s, `<("'`) {
		return
	}
	eachEnclosed(s, func(inner string) {
		if norm, ok := normalize(inner); ok {
			w.sink(Ref{Raw: inner, Path: norm, Where: strings.Join(w.at, ".")})
		}
	})
}

// Journal pages hold whole HTML documents, and matching attributes in them with
// a case-insensitive expression took more time than the rest of the scan. Every
// quoted or bracketed span is offered instead, and the extension check rejects
// what is not a file.
func eachEnclosed(s string, emit func(string)) {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return
			}
			emit(s[i+1 : i+1+j])
			i += j + 1
		case '(':
			j := strings.IndexAny(s[i+1:], `)"'`)
			if j >= 0 && s[i+1+j] == ')' {
				emit(strings.TrimSpace(s[i+1 : i+1+j]))
			}
		}
	}
}

func normalize(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 2048 {
		return "", false
	}
	if strings.ContainsAny(s, "<>\n\r\t") {
		return "", false
	}
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if !mediaExt[lowerExt(s)] {
		return "", false
	}
	if hasScheme(s) || strings.HasPrefix(s, "//") {
		return "", false
	}

	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimPrefix(s, "./")
	if s == "" {
		return "", false
	}
	return path.Clean(s), true
}

// A colon before the first separator marks a URL, a data URI or a drive
// letter. None of them name a file inside the user data directory.
func hasScheme(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ':':
			return true
		case '/', '\\':
			return false
		}
	}
	return false
}

// Lowercasing every string to read its extension allocated once per field.
func lowerExt(s string) string {
	i := strings.LastIndexByte(s, '.')
	if i < 0 || len(s)-i > 8 {
		return ""
	}
	ext := s[i:]
	if strings.ContainsAny(ext, `/\`) {
		return ""
	}
	for k := 1; k < len(ext); k++ {
		if c := ext[k]; c >= 'A' && c <= 'Z' {
			return strings.ToLower(ext)
		}
	}
	return ext
}
