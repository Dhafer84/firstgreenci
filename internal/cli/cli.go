// Package cli wires the command line to the rest of the tool.
//
// Everything the commands need — arguments, streams, environment, whether a
// human is there to answer — is passed in through Environment, so that the
// whole tool can be exercised in a test without touching the real process.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/config"
	"github.com/Dhafer84/firstgreenci/internal/detect"
	"github.com/Dhafer84/firstgreenci/internal/explain"
	"github.com/Dhafer84/firstgreenci/internal/generate"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// Version is the version of the tool, shown by the version command.
//
// A binary built by hand reports "dev". The release machinery replaces this
// at link time with the tag being published, so that the number a user sees
// is the one that was actually released. A version written into the source
// would drift from the tag the first time someone forgot to bump it.
var Version = "dev"

// Exit codes. 2 is reserved for a misuse of the command line, so that a
// script can tell a wrong invocation from a failed run.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// Environment is everything the commands are allowed to touch.
type Environment struct {
	Args      []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	LookupEnv func(string) (string, bool)

	// Interactive reports whether a human can answer a question. When it is
	// false, the tool never blocks on a prompt: it explains what to pass on
	// the command line instead.
	Interactive bool

	// Context carries the cancellation of a long run, so that Ctrl-C stops
	// the container instead of leaving it behind.
	Context context.Context

	// System names the operating system, for the installation guides. It is
	// injected so that a test can check the advice given on Windows without
	// running on Windows.
	System string

	// ConfigPath overrides where the preferences are kept. Tests set it so
	// that they never read or write the real user's file.
	ConfigPath string

	// SystemLanguage reports the language of the operating system, asked
	// for only when no environment variable answers. Tests set it so that
	// their result does not depend on the machine running them.
	SystemLanguage func() string
}

// systemLanguage returns the system language reader to use.
func (e Environment) systemLanguage() func() string {
	if e.SystemLanguage != nil {
		return e.SystemLanguage
	}
	return i18n.SystemLanguage
}

// configPath returns the preferences file to use.
func (e Environment) configPath() (string, error) {
	if e.ConfigPath != "" {
		return e.ConfigPath, nil
	}
	return config.Path()
}

// system returns the operating system to give advice for.
func (e Environment) system() string {
	if e.System == "" {
		return runtime.GOOS
	}
	return e.System
}

// context returns the cancellation context, or a plain one when the caller
// set none.
func (e Environment) context() context.Context {
	if e.Context == nil {
		return context.Background()
	}
	return e.Context
}

// newFlagSet builds a flag set that keeps quiet. The flag package writes its
// own messages, in English and outside the catalogs; they are reported as a
// technical detail behind a translated sentence instead.
func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

// reportUsageError states that the command line was not understood, and shows
// how the command is meant to be used.
func reportUsageError(env Environment, catalog *i18n.Catalog, err error) int {
	fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", err))
	fmt.Fprint(env.Stderr, catalog.T("cli.usage"))
	return exitUsage
}

// Run executes the command line and returns the exit code of the process.
func Run(env Environment) int {
	if len(env.Args) == 0 {
		catalog, code := load(env, "")
		if catalog == nil {
			return code
		}
		fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", catalog.T("cli.error.no_command")))
		fmt.Fprint(env.Stderr, catalog.T("cli.usage"))
		return exitUsage
	}

	switch command := env.Args[0]; command {
	case "init":
		return runInit(env, env.Args[1:])

	case "run":
		return runPipeline(env, env.Args[1:])

	case "doctor":
		return runDoctor(env, env.Args[1:])

	case "version", "--version", "-v":
		catalog, code := load(env, "")
		if catalog == nil {
			return code
		}
		fmt.Fprintln(env.Stdout, catalog.T("cli.version", Version))
		return exitSuccess

	case "help", "--help", "-h":
		catalog, code := load(env, "")
		if catalog == nil {
			return code
		}
		fmt.Fprint(env.Stdout, catalog.T("cli.usage"))
		return exitSuccess

	default:
		catalog, code := load(env, "")
		if catalog == nil {
			return code
		}
		fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", catalog.T("cli.error.unknown_command", command)))
		fmt.Fprint(env.Stderr, catalog.T("cli.usage"))
		return exitUsage
	}
}

// runInit detects the project, generates its workflow and explains it.
func runInit(env Environment, args []string) int {
	flags := newFlagSet("init")

	lang := flags.String("lang", "", "")
	projectType := flags.String("project-type", "", "")
	force := flags.Bool("force", false, "")
	dryRun := flags.Bool("dry-run", false, "")

	parseErr := flags.Parse(args)

	catalog, code := load(env, *lang)
	if catalog == nil {
		return code
	}

	if parseErr != nil {
		return reportUsageError(env, catalog, parseErr)
	}

	if flags.NArg() > 1 {
		fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", catalog.T("cli.error.too_many_arguments", flags.NArg())))
		fmt.Fprint(env.Stderr, catalog.T("cli.usage"))
		return exitUsage
	}

	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}

	project, code := detectProject(env, catalog, root, *projectType)
	if project == nil {
		return code
	}

	content, err := generate.Render(firstgreenci.TemplatesFS, catalog, project)
	if err != nil {
		fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", err))
		return exitFailure
	}

	fmt.Fprintln(env.Stdout, explain.Summary(catalog, project))
	printEvidence(env, catalog, project)

	if *dryRun {
		fmt.Fprintln(env.Stdout)
		fmt.Fprint(env.Stdout, string(content))
		fmt.Fprintln(env.Stdout)
		printSteps(env, catalog, project)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, catalog.T("init.dry_run.note"))
		return exitSuccess
	}

	written, code := writeWorkflow(env, catalog, root, content, *force)
	if written == "" {
		return code
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, catalog.T("init.created", written))
	fmt.Fprintln(env.Stdout)
	printSteps(env, catalog, project)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, catalog.T("init.next", written))

	return exitSuccess
}

// detectProject runs the detection and turns its failures into explanations.
// It returns a nil project when the caller should stop, along with the exit
// code to use.
func detectProject(env Environment, catalog *i18n.Catalog, root, projectType string) (*detect.Project, int) {
	if projectType != "" {
		language, err := detect.ParseLanguage(projectType)
		if err != nil {
			fmt.Fprintln(env.Stderr, catalog.T("detect.error.unknown_project_type", projectType))
			return nil, exitUsage
		}
		project, err := detect.DetectAs(root, language)
		if err != nil {
			return nil, reportDetectionError(env, catalog, root, err)
		}
		return project, exitSuccess
	}

	project, err := detect.Detect(root)
	if err == nil {
		return project, exitSuccess
	}

	// Two languages in one folder is the one case where the tool asks a
	// question rather than choosing for the user: design principle 4.
	var ambiguous *detect.AmbiguousError
	if errors.As(err, &ambiguous) {
		language, ok := askLanguage(env, catalog, ambiguous)
		if !ok {
			return nil, exitFailure
		}
		project, err := detect.DetectAs(root, language)
		if err != nil {
			return nil, reportDetectionError(env, catalog, root, err)
		}
		return project, exitSuccess
	}

	return nil, reportDetectionError(env, catalog, root, err)
}

// reportDetectionError states the cause in plain words and proposes an
// action. Design principle 2: never a raw log on its own.
func reportDetectionError(env Environment, catalog *i18n.Catalog, root string, err error) int {
	var notRecognised *detect.NotRecognisedError
	var missingTest *detect.MissingTestCommandError
	var invalidManifest *detect.InvalidManifestError
	var unknownLanguage *detect.UnknownLanguageError

	switch {
	case errors.As(err, &notRecognised):
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.none", displayPath(root)))
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.none.hint"))

	case errors.As(err, &missingTest):
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.no_test_script", missingTest.Manifest))
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.no_test_script.hint"))

	case errors.As(err, &invalidManifest):
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.invalid_json", invalidManifest.File))
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.invalid_json.hint", invalidManifest.Err))

	case errors.As(err, &unknownLanguage):
		fmt.Fprintln(env.Stderr, catalog.T("detect.error.unknown_project_type", unknownLanguage.Value))

	default:
		fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", err))
	}

	return exitFailure
}

// writeWorkflow saves the workflow, asking before replacing an existing file.
// It returns an empty path when nothing was written.
func writeWorkflow(env Environment, catalog *i18n.Catalog, root string, content []byte, force bool) (string, int) {
	written, err := generate.Write(root, content, force)
	if err == nil {
		return written, exitSuccess
	}

	var exists *generate.FileExistsError
	if errors.As(err, &exists) {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, catalog.T("init.exists", exists.Path))

		if !env.Interactive {
			fmt.Fprintln(env.Stdout, catalog.T("init.exists.hint"))
			return "", exitFailure
		}
		if !askYesNo(env, catalog) {
			fmt.Fprintln(env.Stdout, catalog.T("init.overwrite.declined"))
			return "", exitSuccess
		}

		written, err = generate.Write(root, content, true)
		if err == nil {
			return written, exitSuccess
		}
	}

	var createDir *generate.CreateDirError
	var writeFile *generate.WriteError

	switch {
	case errors.As(err, &createDir):
		fmt.Fprintln(env.Stderr, catalog.T("error.create_dir", createDir.Path, createDir.Err))
	case errors.As(err, &writeFile):
		fmt.Fprintln(env.Stderr, catalog.T("error.write_file", writeFile.Path, writeFile.Err))
	default:
		fmt.Fprintln(env.Stderr, catalog.T("cli.error.prefix", err))
	}

	return "", exitFailure
}

// askLanguage asks which language the pipeline is for, when the folder holds
// both. It accepts the number shown in the question or the language name.
func askLanguage(env Environment, catalog *i18n.Catalog, ambiguous *detect.AmbiguousError) (detect.Language, bool) {
	fmt.Fprintln(env.Stdout, catalog.T("detect.ambiguous.notice"))

	if !env.Interactive {
		fmt.Fprintln(env.Stdout, catalog.T("detect.ambiguous.no_input"))
		return "", false
	}

	fmt.Fprint(env.Stdout, catalog.T("detect.ambiguous.question"))
	answer := strings.ToLower(strings.TrimSpace(readLine(env.Stdin)))

	switch answer {
	case "1":
		return detect.Python, true
	case "2":
		return detect.JavaScript, true
	}

	if language, err := detect.ParseLanguage(answer); err == nil {
		return language, true
	}

	fmt.Fprintln(env.Stdout, catalog.T("detect.ambiguous.hint"))
	return "", false
}

// askYesNo asks whether an existing file may be replaced. Anything other than
// an explicit yes means no: the user's files are never lost by accident.
func askYesNo(env Environment, catalog *i18n.Catalog) bool {
	fmt.Fprint(env.Stdout, catalog.T("init.overwrite.question"))
	answer := strings.ToLower(strings.TrimSpace(readLine(env.Stdin)))

	for _, accepted := range strings.Split(catalog.T("prompt.yes.letters"), ",") {
		if answer == strings.TrimSpace(accepted) {
			return true
		}
	}
	return false
}

func printEvidence(env Environment, catalog *i18n.Catalog, project *detect.Project) {
	lines := explain.Evidence(catalog, project)
	if len(lines) == 0 {
		return
	}

	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, catalog.T("detect.evidence.header"))
	for _, line := range lines {
		fmt.Fprintf(env.Stdout, "  - %s\n", line)
	}
}

func printSteps(env Environment, catalog *i18n.Catalog, project *detect.Project) {
	steps := explain.Steps(catalog, project)

	fmt.Fprintln(env.Stdout, catalog.T("init.steps.header", len(steps)))
	for i, step := range steps {
		fmt.Fprintf(env.Stdout, "  %d. %s — %s\n", i+1, step.Title, step.Description)
	}
}

// load builds the catalog for the language asked for, or for the one the
// environment suggests.
func load(env Environment, explicit string) (*i18n.Catalog, int) {
	lookup := env.LookupEnv
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}

	catalog, err := i18n.Load(firstgreenci.LocalesFS, i18n.Resolve(explicit, lookup, env.systemLanguage()))
	if err != nil {
		// The catalogs are embedded in the binary: failing here means the
		// build itself is broken, and no translation is available to say so.
		fmt.Fprintf(env.Stderr, "firstgreenci: %v\n", err)
		return nil, exitFailure
	}
	return catalog, exitSuccess
}

// readLine reads one answer from the input, without failing on a closed one.
func readLine(input io.Reader) string {
	if input == nil {
		return ""
	}
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return line
}

// displayPath shows the folder the way the user would name it, in full, so
// that "I found nothing" says where nothing was found.
func displayPath(root string) string {
	if absolute, err := filepath.Abs(root); err == nil {
		return absolute
	}
	return root
}
