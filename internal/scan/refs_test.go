package scan

import (
	"sort"
	"testing"
)

func collect(t *testing.T, doc string) []string {
	t.Helper()
	var got []string
	if err := FromDocument([]byte(doc), func(r Ref) { got = append(got, r.Path) }); err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	return got
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFromDocument(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want []string
	}{
		{
			"nested texture and plain img",
			`{"img":"worlds/w/a.webp","background":{"src":"modules/m/maps/b.jpg"}}`,
			[]string{"modules/m/maps/b.jpg", "worlds/w/a.webp"},
		},
		{
			"array of sounds",
			`{"sounds":[{"path":"music/one.ogg"},{"path":"music/two.mp3"}]}`,
			[]string{"music/one.ogg", "music/two.mp3"},
		},
		{
			"html journal body",
			`{"text":{"content":"<p><img src=\"Карты/схема.webp\"></p><a href=\"x/y.pdf\">"}}`,
			[]string{"Карты/схема.webp"},
		},
		{
			"percent encoded cyrillic",
			`{"img":"%D0%9A%D0%B0%D1%80%D1%82%D1%8B/a.webp"}`,
			[]string{"Карты/a.webp"},
		},
		{
			"query string stripped",
			`{"img":"assets/tile.webp?v=1738"}`,
			[]string{"assets/tile.webp"},
		},
		{
			"backslashes normalized",
			`{"img":"assets\\sub\\c.png"}`,
			[]string{"assets/sub/c.png"},
		},
		{
			"external and inline ignored",
			`{"a":"https://example.com/x.png","b":"data:image/png;base64,AAAA","c":"//cdn/x.png"}`,
			nil,
		},
		{
			"non-media strings ignored",
			`{"name":"Bodyguard","type":"npc","desc":"see map.txt"}`,
			nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := collect(t, c.doc)
			if !equal(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestWhereRecordsJSONPath(t *testing.T) {
	var refs []Ref
	doc := `{"background":{"src":"a/b.webp"},"sounds":[{"path":"c.ogg"}]}`
	if err := FromDocument([]byte(doc), func(r Ref) { refs = append(refs, r) }); err != nil {
		t.Fatal(err)
	}

	where := map[string]string{}
	for _, r := range refs {
		where[r.Path] = r.Where
	}
	if where["a/b.webp"] != "background.src" {
		t.Errorf("background.src: got %q", where["a/b.webp"])
	}
	if where["c.ogg"] != "sounds.0.path" {
		t.Errorf("sounds.0.path: got %q", where["c.ogg"])
	}
}

func TestRefsInsideEmbeddedJSON(t *testing.T) {
	doc := `{"key":"levels-3d-preview.settings",
		"value":"{\"particle\":\"modules/levels-3d-preview/assets/particles/dust.png\"}"}`

	got := collect(t, doc)
	want := []string{"modules/levels-3d-preview/assets/particles/dust.png"}
	if !equal(got, want) {
		t.Errorf("refs = %v, want %v", got, want)
	}
}
