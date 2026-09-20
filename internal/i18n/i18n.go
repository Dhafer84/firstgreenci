// Package i18n loads the translation catalogs that ship inside the binary.
//
// Every string shown to a user goes through a catalog, including the comments
// written into the generated workflow: a pipeline generated in French is
// commented in French.
package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os/exec"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DefaultLang is used when no language can be determined. It is also the
// fallback on Windows, where the LANG variables are usually absent.
const DefaultLang = "en"

// Supported lists the languages shipped with the binary.
var Supported = []string{"en", "fr"}

// Catalog holds the messages of a single language.
type Catalog struct {
	lang     string
	messages map[string]string
}

// Load reads the catalog for lang from fsys, which is expected to hold
// "locales/<lang>.json". fsys comes from a go:embed directive, so its paths
// always use forward slashes and must be built with path, not filepath.
func Load(fsys fs.FS, lang string) (*Catalog, error) {
	if !IsSupported(lang) {
		return nil, fmt.Errorf("unsupported language %q, expected one of %s", lang, strings.Join(Supported, ", "))
	}

	name := path.Join("locales", lang+".json")
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read catalog %s: %w", name, err)
	}

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("parse catalog %s: %w", name, err)
	}

	return &Catalog{lang: lang, messages: messages}, nil
}

// Lang reports the language of the catalog.
func (c *Catalog) Lang() string { return c.lang }

// T returns the message stored under key, with args substituted through
// fmt.Sprintf. A missing key returns the key itself: a rough message is
// easier to diagnose than an empty line, and never panics in front of a user.
func (c *Catalog) T(key string, args ...any) string {
	message, ok := c.messages[key]
	if !ok {
		return key
	}
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

// Has reports whether key exists in the catalog. Tests use it to keep the
// catalogs and the code in step.
func (c *Catalog) Has(key string) bool {
	_, ok := c.messages[key]
	return ok
}

// Keys returns every key of the catalog, sorted.
func (c *Catalog) Keys() []string {
	keys := make([]string, 0, len(c.messages))
	for key := range c.messages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// IsSupported reports whether lang ships with the binary.
func IsSupported(lang string) bool {
	for _, candidate := range Supported {
		if candidate == lang {
			return true
		}
	}
	return false
}

// systemLanguageTimeout bounds the one subprocess this package may start. A
// system that does not answer must never freeze the tool.
const systemLanguageTimeout = 2 * time.Second

// SystemLanguage reports the language the operating system is set to, or an
// empty string when it cannot be told.
//
// It reads AppleLanguages, the list of interface languages, and not
// AppleLocale. That distinction cost a released bug: AppleLocale describes
// the region and the date and number formats, and the two disagree as soon
// as someone picks a language from one country and a region from another. A
// Mac whose interface is English can report AppleLocale fr_TN, and reading
// it answered a English-speaking user in French.
//
// Only macOS is covered. Windows is left out on purpose: reading its
// language means starting PowerShell, which would cost about half a second
// on every command.
func SystemLanguage() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), systemLanguageTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, "defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return ""
	}
	return firstAppleLanguage(string(output))
}

// firstAppleLanguage picks the preferred language out of what defaults
// prints, which is a property list array:
//
//	(
//	    "en-US",
//	    "en-GB"
//	)
//
// Values come quoted or bare depending on their content, so both are
// accepted. Anything unreadable yields an empty string, and English applies.
func firstAppleLanguage(output string) string {
	for _, line := range strings.Split(output, "\n") {
		entry := strings.TrimSpace(line)
		if entry == "" || entry == "(" || entry == ")" {
			continue
		}

		entry = strings.TrimSuffix(entry, ",")
		entry = strings.Trim(entry, `"`)
		return strings.TrimSpace(entry)
	}
	return ""
}

// Resolve picks the language to use, in order of decreasing priority: the
// --lang flag, FIRSTGREENCI_LANG, the usual POSIX locale variables, and
// finally the language of the system itself.
//
// lookupEnv and systemLanguage are injected so that the resolution can be
// tested for every system without running on it.
func Resolve(explicit string, lookupEnv func(string) (string, bool), systemLanguage func() string) string {
	if lang, ok := normalize(explicit); ok {
		return lang
	}

	for _, name := range []string{"FIRSTGREENCI_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		value, found := lookupEnv(name)
		if !found {
			continue
		}
		if lang, ok := normalize(value); ok {
			return lang
		}
	}

	// Last resort before English: ask the system. This is where the
	// subprocess is paid for, and only here — a machine that sets LANG never
	// reaches this line.
	if systemLanguage != nil {
		if lang, ok := normalize(systemLanguage()); ok {
			return lang
		}
	}

	return DefaultLang
}

// normalize turns a locale such as "fr_FR.UTF-8" or "FR" into a supported
// language tag. It reports false when the value names no supported language.
func normalize(value string) (string, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", false
	}

	// Cut off the territory and the encoding: fr_FR.UTF-8 -> fr.
	for _, separator := range []string{".", "@", "_", "-"} {
		if base, _, found := strings.Cut(value, separator); found {
			value = base
		}
	}

	if !IsSupported(value) {
		return "", false
	}
	return value, true
}
