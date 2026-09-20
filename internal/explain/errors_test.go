package explain_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/explain"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
	"github.com/Dhafer84/firstgreenci/internal/run"
)

// diagnose replays a recorded act run and explains it, exactly as the command
// line does.
func diagnose(t *testing.T, recording, lang string) explain.Diagnosis {
	t.Helper()

	file, err := os.Open(filepath.Join("..", "..", "testdata", "act", recording))
	if err != nil {
		t.Fatalf("open the recording: %v", err)
	}
	defer file.Close()

	parser := run.NewParser()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		parser.Line(scanner.Text())
	}

	catalog, err := i18n.Load(firstgreenci.LocalesFS, lang)
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return explain.DiagnoseRun(catalog, parser.Result())
}

// TestDiagnoseRealFailures works on act runs recorded on a real machine. Each
// case is a failure a beginner actually meets.
func TestDiagnoseRealFailures(t *testing.T) {
	tests := []struct {
		name       string
		recording  string
		inCause    string
		inAction   string
		wantsKnown bool
	}{
		{
			name:       "a test fails",
			recording:  "red.json.log",
			inCause:    "tests/test_addition.py::test_addition",
			inAction:   "firstgreenci run",
			wantsKnown: true,
		},
		{
			name:       "a dependency is missing from the list",
			recording:  "missingdep.json.log",
			inCause:    "requests",
			inAction:   "requirements.txt",
			wantsKnown: true,
		},
		{
			name:       "no test was found",
			recording:  "notests.json.log",
			inCause:    "aucun test",
			inAction:   "tests/",
			wantsKnown: true,
		},
		{
			name:       "docker went away",
			recording:  "docker-unreachable.json.log",
			inCause:    "Docker",
			inAction:   "firstgreenci doctor",
			wantsKnown: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := diagnose(t, test.recording, "fr")

			if got.Known != test.wantsKnown {
				t.Errorf("Known = %v, want %v (cause: %q)", got.Known, test.wantsKnown, got.Cause)
			}
			if !strings.Contains(got.Cause, test.inCause) {
				t.Errorf("the cause does not mention %q:\n%s", test.inCause, got.Cause)
			}
			if !strings.Contains(got.Action, test.inAction) {
				t.Errorf("the action does not mention %q:\n%s", test.inAction, got.Action)
			}

			// Design principle 2: a cause and an action, never a raw log on
			// its own.
			if got.Cause == "" || got.Action == "" {
				t.Error("a failure was explained without both a cause and an action")
			}
		})
	}
}

// TestDiagnoseNeverShowsGoFormattingComplaints is the guard against a message
// being handed more arguments than it has room for: Go would print its own
// complaint in the middle of a French sentence.
func TestDiagnoseNeverShowsGoFormattingComplaints(t *testing.T) {
	recordings := []string{"red.json.log", "missingdep.json.log", "notests.json.log", "docker-unreachable.json.log", "green.json.log"}

	for _, lang := range i18n.Supported {
		for _, recording := range recordings {
			got := diagnose(t, recording, lang)

			for name, text := range map[string]string{"cause": got.Cause, "action": got.Action} {
				if strings.Contains(text, "%!") {
					t.Errorf("%s in %s, %s: wrong number of arguments: %q", name, lang, recording, text)
				}
				if strings.HasPrefix(text, "pipeline.error.") {
					t.Errorf("%s in %s, %s: raw key shown: %q", name, lang, recording, text)
				}
			}
		}
	}
}

// TestDiagnoseAdmitsWhatItDoesNotKnow checks the honest fallback. It is what
// feeds the list of messages left to translate.
func TestDiagnoseAdmitsWhatItDoesNotKnow(t *testing.T) {
	catalog, err := i18n.Load(firstgreenci.LocalesFS, "fr")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	parser := run.NewParser()
	for _, line := range []string{
		`{"level":"info","msg":"something nobody has ever seen","step":"Lancer les tests","time":"2026-09-19T23:47:24+01:00"}`,
		`{"level":"info","msg":"  ❌  Failure - Main Lancer les tests [1s]","step":"Lancer les tests","stepResult":"failure","time":"2026-09-19T23:47:25+01:00"}`,
		`{"level":"info","msg":"🏁  Job failed","jobResult":"failure","time":"2026-09-19T23:47:25+01:00"}`,
	} {
		parser.Line(line)
	}

	got := explain.DiagnoseRun(catalog, parser.Result())

	if got.Known {
		t.Error("an unknown error was reported as understood")
	}
	if !strings.Contains(got.Action, "github.com/Dhafer84/firstgreenci/issues") {
		t.Errorf("the fallback does not invite a report:\n%s", got.Action)
	}
	if len(got.Excerpt) == 0 {
		t.Error("no log excerpt was kept, so the user is left with nothing to read")
	}
	if !strings.Contains(strings.Join(got.Excerpt, "\n"), "something nobody has ever seen") {
		t.Errorf("the excerpt does not hold what the step printed: %q", got.Excerpt)
	}
}
