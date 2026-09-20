package cli_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Dhafer84/firstgreenci/internal/cli"
	"github.com/Dhafer84/firstgreenci/internal/config"
	runner "github.com/Dhafer84/firstgreenci/internal/run"
)

// standInTools builds stand-ins for docker and act, puts them on the PATH,
// and returns the folder holding them. The whole run command can then be
// driven without Docker, without act and without the network.
func standInTools(t *testing.T, include ...string) string {
	t.Helper()

	folder := t.TempDir()
	for _, tool := range include {
		source := map[string]string{
			"docker": "fakedocker",
			"act":    "fakeact",
		}[tool]

		binary := filepath.Join(folder, tool)
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}

		build := exec.Command("go", "build", "-o", binary, "../run/testdata/"+source)
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build the %s stand-in: %v\n%s", tool, err, output)
		}
	}

	t.Setenv("PATH", folder)
	return folder
}

// runCommand drives the command line with the stand-ins in place.
func runCommand(t *testing.T, args ...string) result {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Environment{
		Args:      args,
		Stdout:    &stdout,
		Stderr:    &stderr,
		LookupEnv: func(string) (string, bool) { return "", false },
		// The machine running the tests must not decide their language.
		SystemLocale: func() string { return "" },
		System:       "darwin",
		// Never read or write the real user's preferences.
		ConfigPath: filepath.Join(t.TempDir(), "config.json"),
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// projectWithWorkflow copies a sample project and generates its workflow.
func projectWithWorkflow(t *testing.T, name string) string {
	t.Helper()

	root := sampleProject(t, name)
	if got := run(t, "", false, "init", "--lang", "fr", root); got.code != 0 {
		t.Fatalf("init: exit code %d\n%s", got.code, got.stderr)
	}
	return root
}

func TestRunWithoutWorkflowSaysWhatToDo(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")

	got := runCommand(t, "run", "--lang", "fr", root)

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1", got.code)
	}
	if !strings.Contains(got.stderr, "firstgreenci init") {
		t.Errorf("the message does not say which command to run:\n%s", got.stderr)
	}
	// run must never create a file behind the user's back.
	if _, err := os.Stat(workflowPath(root)); !os.IsNotExist(err) {
		t.Error("run created a workflow file, which is not its job")
	}
}

func TestRunWhenDockerIsNotInstalled(t *testing.T) {
	root := projectWithWorkflow(t, "python-pip-pytest")
	standInTools(t) // nothing on the PATH at all

	got := runCommand(t, "run", "--lang", "fr", root)

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1", got.code)
	}
	if !strings.Contains(got.stderr, "docker.com") {
		t.Errorf("the installation guide was not shown:\n%s", got.stderr)
	}
}

func TestRunWhenDockerIsNotRunning(t *testing.T) {
	root := projectWithWorkflow(t, "python-pip-pytest")
	standInTools(t, "docker", "act")
	t.Setenv("FAKE_DOCKER_INFO_EXIT", "1")
	t.Setenv("FAKE_DOCKER_INFO_STDERR", "Cannot connect to the Docker daemon.")

	got := runCommand(t, "run", "--lang", "fr", root)

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1", got.code)
	}
	if !strings.Contains(got.stderr, "Applications") {
		t.Errorf("the guide for starting Docker was not shown:\n%s", got.stderr)
	}
}

func TestRunWhenActIsMissing(t *testing.T) {
	root := projectWithWorkflow(t, "python-pip-pytest")
	standInTools(t, "docker")
	t.Setenv("FAKE_DOCKER_INFO_EXIT", "0")

	got := runCommand(t, "run", "--lang", "fr", root)

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1", got.code)
	}
	if !strings.Contains(got.stderr, "brew install act") {
		t.Errorf("the guide for installing act was not shown:\n%s", got.stderr)
	}
}

// TestRunReplaysRecordedRuns drives the whole command with the output of real
// act runs, recorded in testdata/act. It is the closest thing to the real
// experience that a test can reach without Docker.
func TestRunReplaysRecordedRuns(t *testing.T) {
	tests := []struct {
		name      string
		recording string
		exit      string
		code      int
		says      []string
		saysNot   []string
	}{
		{
			name:      "green pipeline",
			recording: "green.json.log",
			exit:      "0",
			code:      0,
			says:      []string{"Votre pipeline est vert.", "✓ Récupérer le code", "✓ Lancer les tests"},
			saysNot:   []string{"Cause"},
		},
		{
			name:      "a test failed",
			recording: "red.json.log",
			exit:      "1",
			code:      1,
			says: []string{
				"Votre pipeline est rouge.",
				"✗ Lancer les tests",
				"tests/test_addition.py::test_addition",
				"firstgreenci run --verbose",
			},
		},
		{
			name:      "a dependency is missing",
			recording: "missingdep.json.log",
			exit:      "1",
			code:      1,
			says:      []string{"requests", "requirements.txt"},
		},
		{
			name:      "no test was found",
			recording: "notests.json.log",
			exit:      "1",
			code:      1,
			says:      []string{"aucun test"},
		},
		{
			name:      "docker went away during the run",
			recording: "docker-unreachable.json.log",
			exit:      "1",
			code:      1,
			says:      []string{"Docker", "firstgreenci doctor"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := projectWithWorkflow(t, "python-pip-pytest")
			standInTools(t, "docker", "act")

			recording, err := filepath.Abs(filepath.Join("..", "..", "testdata", "act", test.recording))
			if err != nil {
				t.Fatalf("locate the recording: %v", err)
			}
			t.Setenv("FAKE_DOCKER_INFO_EXIT", "0")
			t.Setenv("FAKE_DOCKER_IMAGE_EXIT", "0")
			t.Setenv("FAKE_ACT_RECORDING", recording)
			t.Setenv("FAKE_ACT_EXIT", test.exit)

			got := runCommand(t, "run", "--lang", "fr", root)

			if got.code != test.code {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", got.code, test.code, got.stdout, got.stderr)
			}
			for _, expected := range test.says {
				if !strings.Contains(got.stdout, expected) {
					t.Errorf("the report does not contain %q:\n%s", expected, got.stdout)
				}
			}
			for _, unexpected := range test.saysNot {
				if strings.Contains(got.stdout, unexpected) {
					t.Errorf("the report should not contain %q:\n%s", unexpected, got.stdout)
				}
			}

			// act's own steps are noise when they go well.
			if strings.Contains(got.stdout, "Set up job") && test.recording != "docker-unreachable.json.log" {
				t.Errorf("one of act's internal steps was shown:\n%s", got.stdout)
			}
		})
	}
}

// TestRunAnnouncesTheDownload checks the message shown before a download of
// more than a gigabyte, so that waiting is not mistaken for freezing.
func TestRunAnnouncesTheDownload(t *testing.T) {
	root := projectWithWorkflow(t, "python-pip-pytest")
	standInTools(t, "docker", "act")

	recording, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "act", "green.json.log"))
	t.Setenv("FAKE_DOCKER_INFO_EXIT", "0")
	// The image is not on the machine.
	t.Setenv("FAKE_DOCKER_IMAGE_EXIT", "1")
	t.Setenv("FAKE_ACT_RECORDING", recording)

	got := runCommand(t, "run", "--lang", "fr", root)

	if !strings.Contains(got.stdout, "Téléchargement de l'image") {
		t.Errorf("the download was not announced:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "n'arrive qu'une fois") {
		t.Errorf("the message does not say this happens only once:\n%s", got.stdout)
	}
}

// askImage drives the run command with someone at the keyboard.
func askImage(t *testing.T, answer, configPath string, args ...string) result {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Environment{
		Args:         args,
		Stdin:        strings.NewReader(answer),
		Stdout:       &stdout,
		Stderr:       &stderr,
		LookupEnv:    func(string) (string, bool) { return "", false },
		SystemLocale: func() string { return "" },
		System:       "darwin",
		Interactive:  true,
		ConfigPath:   configPath,
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// TestImageIsAskedOnceAndRemembered covers the one question the tool allows
// itself. It is a deliberate exception to "ask only when detection fails":
// the choice commits more than a gigabyte of download.
func TestImageIsAskedOnceAndRemembered(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{name: "the light image", answer: "2\n", want: runner.SmallImage},
		{name: "the faithful image", answer: "1\n", want: runner.FaithfulImage},
		{name: "no answer keeps the faithful one", answer: "\n", want: runner.FaithfulImage},
		{name: "an answer not understood keeps the faithful one", answer: "peut-être\n", want: runner.FaithfulImage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := projectWithWorkflow(t, "python-pip-pytest")
			standInTools(t, "docker", "act")

			recording, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "act", "green.json.log"))
			t.Setenv("FAKE_DOCKER_INFO_EXIT", "0")
			t.Setenv("FAKE_DOCKER_IMAGE_EXIT", "0")
			t.Setenv("FAKE_ACT_RECORDING", recording)

			configPath := filepath.Join(t.TempDir(), "config.json")
			got := askImage(t, test.answer, configPath, "run", "--lang", "fr", root)

			if !strings.Contains(got.stdout, "image de conteneur") {
				t.Errorf("the question was not asked:\n%s", got.stdout)
			}

			// The answer has to survive to the next run.
			saved, err := config.Load(configPath)
			if err != nil {
				t.Fatalf("read the preferences back: %v", err)
			}
			if saved.RunnerImage != test.want {
				t.Errorf("the remembered image is %q, want %q", saved.RunnerImage, test.want)
			}

			// A second run must not ask again.
			again := askImage(t, "", configPath, "run", "--lang", "fr", root)
			if strings.Contains(again.stdout, "image de conteneur") {
				t.Errorf("the question was asked a second time:\n%s", again.stdout)
			}
		})
	}
}

// TestImageFlagOverridesWithoutRemembering checks that --image is for one run
// and never overwrites a remembered choice.
func TestImageFlagOverridesWithoutRemembering(t *testing.T) {
	root := projectWithWorkflow(t, "python-pip-pytest")
	standInTools(t, "docker", "act")

	recording, _ := filepath.Abs(filepath.Join("..", "..", "testdata", "act", "green.json.log"))
	t.Setenv("FAKE_DOCKER_INFO_EXIT", "0")
	t.Setenv("FAKE_DOCKER_IMAGE_EXIT", "0")
	t.Setenv("FAKE_ACT_RECORDING", recording)

	configPath := filepath.Join(t.TempDir(), "config.json")
	got := askImage(t, "", configPath, "run", "--lang", "fr", "--image", "example/image:latest", root)

	if strings.Contains(got.stdout, "image de conteneur") {
		t.Errorf("--image was given, yet the question was asked:\n%s", got.stdout)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("--image wrote a preference, although it is meant for one run only")
	}
}

// TestInterruptedRunCleansUp covers the promise made when Ctrl-C is pressed.
// act does not always get far enough to remove the container it just created,
// so the tool removes it and checks, rather than claiming it did.
func TestInterruptedRunCleansUp(t *testing.T) {
	tests := []struct {
		name      string
		removable bool
		says      string
	}{
		{name: "the container is removed", removable: true, says: "Rien n'a été laissé en marche"},
		{name: "the container survives", removable: false, says: "docker rm --force act-demo"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := projectWithWorkflow(t, "python-pip-pytest")
			standInTools(t, "docker", "act")

			t.Setenv("FAKE_DOCKER_INFO_EXIT", "0")
			t.Setenv("FAKE_DOCKER_IMAGE_EXIT", "0")
			// Nothing before the run, one container of ours afterwards.
			t.Setenv("FAKE_DOCKER_PS_COUNTER", filepath.Join(t.TempDir(), "calls"))
			t.Setenv("FAKE_DOCKER_PS_FIRST", "")
			t.Setenv("FAKE_DOCKER_PS_LATER", "act-demo")
			if test.removable {
				t.Setenv("FAKE_DOCKER_RM_EXIT", "0")
			} else {
				t.Setenv("FAKE_DOCKER_RM_EXIT", "1")
			}

			// The run has to be cancelled while act is going, not before:
			// cancelling earlier would stop the tool at the Docker check.
			// act says when it has started, so the moment is exact rather
			// than raced against a timer.
			started := filepath.Join(t.TempDir(), "started")
			t.Setenv("FAKE_ACT_STARTED_FILE", started)
			t.Setenv("FAKE_ACT_SLEEP_MS", "5000")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go cancelWhenStarted(started, cancel)

			var stdout, stderr bytes.Buffer
			code := cli.Run(cli.Environment{
				Args:       []string{"run", "--lang", "fr", "--image", "example/image:latest", root},
				Stdout:     &stdout,
				Stderr:     &stderr,
				LookupEnv:  func(string) (string, bool) { return "", false },
				System:     "darwin",
				Context:    ctx,
				ConfigPath: filepath.Join(t.TempDir(), "config.json"),
			})

			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stdout.String(), test.says) {
				t.Errorf("the message does not contain %q:\nstdout:\n%s\nstderr:\n%s", test.says, stdout.String(), stderr.String())
			}
		})
	}
}

// cancelWhenStarted waits for act to report that it is running, then cancels.
// It gives up after a few seconds so that a failing test never hangs.
func cancelWhenStarted(marker string, cancel context.CancelFunc) {
	for waited := time.Duration(0); waited < 10*time.Second; waited += 5 * time.Millisecond {
		if _, err := os.Stat(marker); err == nil {
			cancel()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
}
