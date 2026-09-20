package i18n_test

import (
	"testing"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// TestCatalogsHaveTheSameKeys guards the promise that every language shows the
// same messages. A key added to one catalog and forgotten in the other would
// silently show a raw key to half the users.
func TestCatalogsHaveTheSameKeys(t *testing.T) {
	reference, err := i18n.Load(firstgreenci.LocalesFS, i18n.DefaultLang)
	if err != nil {
		t.Fatalf("load reference catalog: %v", err)
	}
	referenceKeys := reference.Keys()

	for _, lang := range i18n.Supported {
		if lang == i18n.DefaultLang {
			continue
		}

		catalog, err := i18n.Load(firstgreenci.LocalesFS, lang)
		if err != nil {
			t.Fatalf("load catalog %s: %v", lang, err)
		}

		for _, key := range referenceKeys {
			if !catalog.Has(key) {
				t.Errorf("catalog %s is missing key %q, present in %s", lang, key, i18n.DefaultLang)
			}
		}
		for _, key := range catalog.Keys() {
			if !reference.Has(key) {
				t.Errorf("catalog %s has key %q, absent from %s", lang, key, i18n.DefaultLang)
			}
		}
	}
}

func TestLoadRejectsUnsupportedLanguage(t *testing.T) {
	if _, err := i18n.Load(firstgreenci.LocalesFS, "de"); err == nil {
		t.Fatal("expected an error for an unsupported language, got none")
	}
}

func TestTranslate(t *testing.T) {
	catalog, err := i18n.Load(firstgreenci.LocalesFS, "fr")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	tests := []struct {
		name string
		key  string
		args []any
		want string
	}{
		{
			name: "plain message",
			key:  "language.python",
			want: "Python",
		},
		{
			name: "message with arguments",
			key:  "init.created",
			args: []any{".github/workflows/ci.yml"},
			want: "Fichier créé   : .github/workflows/ci.yml",
		},
		{
			name: "missing key falls back to the key itself",
			key:  "does.not.exist",
			want: "does.not.exist",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := catalog.T(test.key, test.args...); got != test.want {
				t.Errorf("T(%q) = %q, want %q", test.key, got, test.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	env := func(values map[string]string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			value, ok := values[name]
			return value, ok
		}
	}

	tests := []struct {
		name     string
		explicit string
		values   map[string]string
		want     string
	}{
		{
			name:     "the flag wins over everything",
			explicit: "en",
			values:   map[string]string{"LANG": "fr_FR.UTF-8"},
			want:     "en",
		},
		{
			name:   "the dedicated variable wins over the locale",
			values: map[string]string{"FIRSTGREENCI_LANG": "fr", "LANG": "en_US.UTF-8"},
			want:   "fr",
		},
		{
			name:   "a POSIX locale is reduced to its language",
			values: map[string]string{"LANG": "fr_FR.UTF-8"},
			want:   "fr",
		},
		{
			name:   "LC_ALL wins over LANG",
			values: map[string]string{"LC_ALL": "fr_CA", "LANG": "en_GB"},
			want:   "fr",
		},
		{
			name:   "an unsupported locale falls back to the default",
			values: map[string]string{"LANG": "de_DE.UTF-8"},
			want:   i18n.DefaultLang,
		},
		{
			name:   "no variable at all, as on Windows, falls back to the default",
			values: map[string]string{},
			want:   i18n.DefaultLang,
		},
		{
			name:     "an unsupported flag value does not shadow the environment",
			explicit: "de",
			values:   map[string]string{"LANG": "fr_FR.UTF-8"},
			want:     "fr",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := i18n.Resolve(test.explicit, env(test.values)); got != test.want {
				t.Errorf("Resolve(%q, %v) = %q, want %q", test.explicit, test.values, got, test.want)
			}
		})
	}
}
