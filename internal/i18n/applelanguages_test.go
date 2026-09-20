package i18n

import "testing"

// TestFirstAppleLanguage works on the shapes defaults actually prints. The
// real command is never run here, so the test behaves the same on the three
// systems.
func TestFirstAppleLanguage(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "the usual quoted list",
			output: `(
    "en-US",
    "en-GB",
    "fr-TN"
)`,
			want: "en-US",
		},
		{
			name: "a French machine",
			output: `(
    "fr-FR",
    "en-US"
)`,
			want: "fr-FR",
		},
		{
			// defaults leaves simple values unquoted.
			name: "bare entries",
			output: `(
    en,
    fr
)`,
			want: "en",
		},
		{
			name:   "a single entry on one line",
			output: "(\n    \"fr-TN\"\n)",
			want:   "fr-TN",
		},
		{
			name:   "an empty list",
			output: "(\n)",
			want:   "",
		},
		{
			name:   "nothing at all",
			output: "",
			want:   "",
		},
		{
			name:   "only whitespace",
			output: "   \n\n  ",
			want:   "",
		},
		{
			// A truncated read yields whatever it yields; Resolve then
			// rejects it, since it names no language we ship.
			name:   "output that makes no sense",
			output: "domain not found",
			want:   "domain not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := firstAppleLanguage(test.output); got != test.want {
				t.Errorf("firstAppleLanguage() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestNonsenseIsRejected is the other half of the contract: whatever the
// parser lets through, only a language the tool ships can win.
func TestNonsenseIsRejected(t *testing.T) {
	nothing := func(string) (string, bool) { return "", false }

	tests := []struct {
		system string
		want   string
	}{
		{system: "domain not found", want: DefaultLang},
		{system: "fr-TN", want: "fr"},
		{system: "en-US", want: DefaultLang},
		{system: "", want: DefaultLang},
	}

	for _, test := range tests {
		t.Run(test.system, func(t *testing.T) {
			got := Resolve("", nothing, func() string { return test.system })
			if got != test.want {
				t.Errorf("Resolve with system %q = %q, want %q", test.system, got, test.want)
			}
		})
	}
}
