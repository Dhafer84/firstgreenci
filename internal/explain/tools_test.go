package explain_test

import (
	"strings"
	"testing"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/explain"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// TestGuidesExistForEverySystem checks that no user, on any system and in any
// language, is told what to do with a raw translation key.
func TestGuidesExistForEverySystem(t *testing.T) {
	guides := map[string]func(*i18n.Catalog, string) string{
		"install docker": explain.DockerInstallGuide,
		"start docker":   explain.DockerStartGuide,
		"install act":    explain.ActInstallGuide,
	}

	// "freebsd" stands for any system that is neither macOS nor Windows and
	// must fall back to the Linux guide.
	for _, goos := range []string{"darwin", "windows", "linux", "freebsd"} {
		for _, lang := range i18n.Supported {
			catalog, err := i18n.Load(firstgreenci.LocalesFS, lang)
			if err != nil {
				t.Fatalf("load catalog %s: %v", lang, err)
			}

			for name, guide := range guides {
				got := guide(catalog, goos)
				if got == "" {
					t.Errorf("%s on %s in %s is empty", name, goos, lang)
				}
				if strings.HasPrefix(got, "install.") || strings.HasPrefix(got, "start.") {
					t.Errorf("%s on %s in %s shows a raw key: %q", name, goos, lang, got)
				}
			}
		}
	}
}
