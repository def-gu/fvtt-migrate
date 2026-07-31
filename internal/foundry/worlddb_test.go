package foundry

import "testing"

func TestSplitKey(t *testing.T) {
	cases := []struct {
		raw, namespace, key string
	}{
		{"!actors!03eULzJAOTs3FIiD", "actors", "03eULzJAOTs3FIiD"},
		{"!scenes.tokens!TRy1aW5EO3SE4mat.SmEYhYWOAq3KefaM", "scenes.tokens", "TRy1aW5EO3SE4mat.SmEYhYWOAq3KefaM"},
		{"!scenes.tokens.delta.items!a.b.c", "scenes.tokens.delta.items", "a.b.c"},
		{"malformed", "", "malformed"},
	}

	for _, c := range cases {
		ns, key := splitKey(c.raw)
		if ns != c.namespace || key != c.key {
			t.Errorf("splitKey(%q) = (%q, %q), want (%q, %q)", c.raw, ns, key, c.namespace, c.key)
		}
	}
}
