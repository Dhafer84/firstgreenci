package run_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/run"
)

// TestArgumentsNeverTouchesPersonalConfiguration pins down the command line.
// Two of these flags are promises rather than details: -P keeps act from
// asking its own question and writing into ~/.actrc, and --pull=false keeps a
// cached image from being re-fetched on every run.
func TestArguments(t *testing.T) {
	tests := []struct {
		name    string
		options run.Options
		want    []string
	}{
		{
			name: "image already on the machine",
			options: run.Options{
				Workflow: filepath.Join(".github", "workflows", "ci.yml"),
				Image:    "catthehacker/ubuntu:act-latest",
				Pull:     false,
			},
			want: []string{
				"push",
				"-W", ".github/workflows/ci.yml",
				"-P", "ubuntu-latest=catthehacker/ubuntu:act-latest",
				"--pull=false",
				"--rm",
				"--json",
			},
		},
		{
			name: "image still to be downloaded",
			options: run.Options{
				Workflow: filepath.Join(".github", "workflows", "ci.yml"),
				Image:    "node:16-buster-slim",
				Pull:     true,
			},
			want: []string{
				"push",
				"-W", ".github/workflows/ci.yml",
				"-P", "ubuntu-latest=node:16-buster-slim",
				"--pull=true",
				"--rm",
				"--json",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.options.Arguments()

			if len(got) != len(test.want) {
				t.Fatalf("Arguments() = %v, want %v", got, test.want)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("Arguments()[%d] = %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}

// TestArgumentsUseForwardSlashes checks the workflow path given to act, which
// runs inside a Linux container whatever the host is.
func TestArgumentsUseForwardSlashes(t *testing.T) {
	options := run.Options{Workflow: filepath.Join(".github", "workflows", "ci.yml"), Image: "x"}

	for _, argument := range options.Arguments() {
		if strings.Contains(argument, "\\") {
			t.Errorf("argument %q holds a backslash, which act would not read as a path", argument)
		}
	}
}

func TestExecuteStreamsEveryLine(t *testing.T) {
	act := &run.Act{Path: buildStandIn(t, "fakeact")}

	t.Setenv("FAKE_ACT_STDOUT", "first line|second line")
	t.Setenv("FAKE_ACT_STDERR", "a log line")
	t.Setenv("FAKE_ACT_TAIL", "line without a newline")
	t.Setenv("FAKE_ACT_EXIT", "0")

	var lines []string
	err := act.Execute(context.Background(), run.Options{Root: t.TempDir(), Image: "x"}, func(line string) {
		lines = append(lines, line)
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, want := range []string{"first line", "second line", "a log line", "line without a newline"} {
		if !contains(lines, want) {
			t.Errorf("the line %q was never delivered; got %q", want, lines)
		}
	}
}

// TestExecuteReportsTheExitStatus matters because a red pipeline also exits
// non-zero: the caller has to be able to tell the two apart.
func TestExecuteReportsTheExitStatus(t *testing.T) {
	act := &run.Act{Path: buildStandIn(t, "fakeact")}

	t.Setenv("FAKE_ACT_STDOUT", "something happened")
	t.Setenv("FAKE_ACT_EXIT", "1")

	err := act.Execute(context.Background(), run.Options{Root: t.TempDir(), Image: "x"}, func(string) {})
	if err == nil {
		t.Fatal("Execute reported success although act exited with 1")
	}
}

func TestExecutePassesTheArgumentsThrough(t *testing.T) {
	act := &run.Act{Path: buildStandIn(t, "fakeact")}

	recorded := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("FAKE_ACT_ARGS_FILE", recorded)

	options := run.Options{
		Root:     t.TempDir(),
		Workflow: filepath.Join(".github", "workflows", "ci.yml"),
		Image:    "catthehacker/ubuntu:act-latest",
	}
	if err := act.Execute(context.Background(), options, func(string) {}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	content, err := os.ReadFile(recorded)
	if err != nil {
		t.Fatalf("read the recorded arguments: %v", err)
	}
	if !strings.Contains(string(content), "-P\nubuntu-latest=catthehacker/ubuntu:act-latest") {
		t.Errorf("act did not receive an explicit image, so it would write into the user's ~/.actrc:\n%s", content)
	}
}

func TestFindActReportsAbsence(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := run.FindAct()

	var notInstalled *run.ActNotInstalledError
	if !errors.As(err, &notInstalled) {
		t.Fatalf("got %T (%v), want *run.ActNotInstalledError", err, err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
