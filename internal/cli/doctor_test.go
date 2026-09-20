package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/cli"
)

// TestDoctorReportsEveryTool runs the real checks against whatever the
// machine has. It cannot assume Docker is installed or running, so it asserts
// what must hold in every case: the three checks are reported, nothing shows
// a raw key, and the exit code agrees with the marks that were printed.
func TestDoctorReportsEveryTool(t *testing.T) {
	tests := []struct {
		lang   string
		labels []string
	}{
		{lang: "fr", labels: []string{"Docker", "act", "Image"}},
		{lang: "en", labels: []string{"Docker", "act", "Image"}},
	}

	for _, test := range tests {
		t.Run(test.lang, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := cli.Run(cli.Environment{
				Args:      []string{"doctor", "--lang", test.lang},
				Stdout:    &stdout,
				Stderr:    &stderr,
				LookupEnv: func(string) (string, bool) { return "", false },
				// The machine running the tests must not decide their language.
				SystemLocale: func() string { return "" },
			})

			output := stdout.String()
			for _, label := range test.labels {
				if !strings.Contains(output, label) {
					t.Errorf("the report does not mention %q:\n%s", label, output)
				}
			}

			for _, prefix := range []string{"doctor.", "install.", "start."} {
				if strings.Contains(output, prefix) {
					t.Errorf("the report shows a raw key starting with %q:\n%s", prefix, output)
				}
			}

			// A problem was reported, so the command must fail; and if it
			// failed, it must have said why.
			hasProblem := strings.Contains(output, "✗")
			if hasProblem && code != 1 {
				t.Errorf("a problem was reported but the exit code is %d, want 1", code)
			}
			if !hasProblem && code != 0 {
				t.Errorf("no problem was reported but the exit code is %d, want 0", code)
			}
		})
	}
}

func TestDoctorRejectsUnknownFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Environment{
		Args:      []string{"doctor", "--turbo"},
		Stdout:    &stdout,
		Stderr:    &stderr,
		LookupEnv: func(string) (string, bool) { return "", false },
		// The machine running the tests must not decide their language.
		SystemLocale: func() string { return "" },
	})

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "firstgreenci") {
		t.Errorf("stderr does not show how to use the command:\n%s", stderr.String())
	}
}
