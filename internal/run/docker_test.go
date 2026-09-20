package run_test

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/run"
)

// buildStandIn compiles one of the stand-in commands under testdata and
// returns its path. Compiling rather than shipping a shell script keeps the
// tests working on Windows.
func buildStandIn(t *testing.T, name string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	build := exec.Command("go", "build", "-o", binary, "./testdata/"+name)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the %s stand-in: %v\n%s", name, err, output)
	}
	return binary
}

func TestCheckDaemon(t *testing.T) {
	docker := &run.Docker{Path: buildStandIn(t, "fakedocker")}

	tests := []struct {
		name       string
		exit       string
		stderr     string
		wantErr    bool
		wantDetail string
	}{
		{
			name: "the daemon answers",
			exit: "0",
		},
		{
			name:       "the daemon is not running",
			exit:       "1",
			stderr:     "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
			wantErr:    true,
			wantDetail: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
		},
		{
			name:       "only the last line is kept",
			exit:       "1",
			stderr:     "WARNING: something older\nCannot connect to the Docker daemon.",
			wantErr:    true,
			wantDetail: "Cannot connect to the Docker daemon.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("FAKE_DOCKER_INFO_EXIT", test.exit)
			t.Setenv("FAKE_DOCKER_INFO_STDERR", test.stderr)

			err := docker.CheckDaemon(context.Background())

			if !test.wantErr {
				if err != nil {
					t.Fatalf("CheckDaemon: %v", err)
				}
				return
			}

			var notRunning *run.DockerNotRunningError
			if !errors.As(err, &notRunning) {
				t.Fatalf("got %T (%v), want *run.DockerNotRunningError", err, err)
			}
			if notRunning.Detail != test.wantDetail {
				t.Errorf("Detail = %q, want %q", notRunning.Detail, test.wantDetail)
			}
		})
	}
}

func TestHasImage(t *testing.T) {
	docker := &run.Docker{Path: buildStandIn(t, "fakedocker")}

	tests := []struct {
		name string
		exit string
		want bool
	}{
		{name: "the image is on the machine", exit: "0", want: true},
		{name: "the image has to be downloaded", exit: "1", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("FAKE_DOCKER_IMAGE_EXIT", test.exit)

			if got := docker.HasImage(context.Background(), "catthehacker/ubuntu:act-latest"); got != test.want {
				t.Errorf("HasImage = %v, want %v", got, test.want)
			}
		})
	}
}

// TestFindDockerReportsAbsence checks the message given when docker is not
// installed at all, which is what 2 people out of 5 hit during the interviews.
func TestFindDockerReportsAbsence(t *testing.T) {
	// An empty PATH makes the lookup fail whatever the machine has.
	t.Setenv("PATH", t.TempDir())

	_, err := run.FindDocker()

	var notInstalled *run.DockerNotInstalledError
	if !errors.As(err, &notInstalled) {
		t.Fatalf("got %T (%v), want *run.DockerNotInstalledError", err, err)
	}
}
