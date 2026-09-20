package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Dhafer84/firstgreenci/internal/config"
	"github.com/Dhafer84/firstgreenci/internal/explain"
	"github.com/Dhafer84/firstgreenci/internal/generate"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
	"github.com/Dhafer84/firstgreenci/internal/run"
)

// stepColumn is where the duration is printed. A longer step name simply
// pushes it right: the steps are announced as they finish, so their widths
// cannot be known in advance.
const stepColumn = 32

// runPipeline runs the generated workflow on the user's own machine and says,
// step by step, what happened.
//
// It never writes a file. A project with no workflow is told which command to
// run, rather than having one appear behind its back.
func runPipeline(env Environment, args []string) int {
	flags := newFlagSet("run")
	lang := flags.String("lang", "", "")
	verbose := flags.Bool("verbose", false, "")
	image := flags.String("image", "", "")
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
		return exitUsage
	}

	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}

	if !generate.Exists(root) {
		fmt.Fprintln(env.Stderr, catalog.T("run.no_workflow", displayPath(root)))
		fmt.Fprintln(env.Stderr, catalog.T("run.no_workflow.hint"))
		return exitFailure
	}

	tools, code := prepareTools(env, catalog)
	if tools == nil {
		return code
	}

	chosen := chooseImage(env, catalog, *image)

	// A missing image means a download of more than a gigabyte. Saying so
	// beforehand is the difference between waiting and thinking it froze.
	cached := tools.docker.HasImage(env.context(), chosen)
	if !cached {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, catalog.T("run.image.downloading", chosen))
		fmt.Fprintln(env.Stdout, catalog.T("run.image.downloading.note"))
	}

	return execute(env, catalog, tools, *verbose, run.Options{
		Root:     root,
		Workflow: generate.WorkflowPath(),
		Image:    chosen,
		// act refreshes the image from the registry on every run unless it
		// is told not to. Once the image is here, that is a needless round
		// trip, and it makes an offline run fail.
		Pull: !cached,
	})
}

// tools holds the two commands a run depends on.
type tools struct {
	docker *run.Docker
	act    *run.Act
}

// prepareTools checks Docker and act, and explains what to do when one is
// missing. It returns nil when the run cannot go ahead.
func prepareTools(env Environment, catalog *i18n.Catalog) (*tools, int) {
	docker, err := run.FindDocker()
	if err != nil {
		fmt.Fprintln(env.Stderr, catalog.T("doctor.docker.label")+" : "+catalog.T("doctor.docker.not_installed"))
		fmt.Fprintln(env.Stderr)
		fmt.Fprintln(env.Stderr, explain.DockerInstallGuide(catalog, env.system()))
		return nil, exitFailure
	}

	if err := docker.CheckDaemon(env.context()); err != nil {
		fmt.Fprintln(env.Stderr, catalog.T("doctor.docker.label")+" : "+catalog.T("doctor.docker.not_running"))
		fmt.Fprintln(env.Stderr)
		fmt.Fprintln(env.Stderr, explain.DockerStartGuide(catalog, env.system()))
		return nil, exitFailure
	}

	act, err := run.FindAct()
	if err != nil {
		fmt.Fprintln(env.Stderr, catalog.T("doctor.act.label")+" : "+catalog.T("doctor.act.not_installed"))
		fmt.Fprintln(env.Stderr)
		fmt.Fprintln(env.Stderr, explain.ActInstallGuide(catalog, env.system()))
		return nil, exitFailure
	}

	return &tools{docker: docker, act: act}, exitSuccess
}

// execute runs the pipeline and reports it.
func execute(env Environment, catalog *i18n.Catalog, tools *tools, verbose bool, options run.Options) int {
	fmt.Fprintln(env.Stdout, catalog.T("run.header.pipeline", options.Workflow))
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, catalog.T("run.starting"))
	fmt.Fprintln(env.Stdout)

	// The containers act is responsible for, before we start. Anything with
	// that name appearing afterwards is ours, and ours alone, to clean up.
	existing := tools.docker.ActContainers(env.context())

	parser := run.NewParser()
	parser.OnStep = func(step run.Step) {
		// act wraps the workflow in steps of its own. They are noise while
		// they go well, and are explained as a preparation failure when they
		// do not.
		if step.Internal && step.Status == run.StatusSuccess {
			return
		}
		// An interrupted step did not fail; saying so would be a lie.
		if env.context().Err() != nil {
			return
		}
		printStep(env, catalog, step)
	}

	// The status act exits with is not read on its own: a red pipeline also
	// exits non-zero. What was parsed says which of the two happened.
	_ = tools.act.Execute(env.context(), options, func(line string) {
		if verbose {
			fmt.Fprintln(env.Stdout, run.HumanLine(line))
		}
		parser.Line(line)
	})
	result := parser.Result()

	fmt.Fprintln(env.Stdout)

	// Ctrl-C during a run. act does not always get far enough to remove the
	// container it just created, so we check rather than promise.
	if env.context().Err() != nil {
		if leftover := cleanUp(tools, existing); leftover != "" {
			fmt.Fprintln(env.Stdout, catalog.T("run.interrupted.leftover", leftover))
			return exitFailure
		}
		fmt.Fprintln(env.Stdout, catalog.T("run.interrupted"))
		return exitFailure
	}

	if result.Succeeded() {
		fmt.Fprintln(env.Stdout, catalog.T("run.green"))
		return exitSuccess
	}

	if !result.Ran() {
		// act never announced a verdict, so the problem is with the tooling
		// rather than with the user's code.
		fmt.Fprintln(env.Stdout, catalog.T("run.tool_failure"))
	} else {
		fmt.Fprintln(env.Stdout, catalog.T("run.red"))
	}

	report(env, catalog, explain.DiagnoseRun(catalog, result), verbose)
	return exitFailure
}

// cleanUp removes the containers that appeared during our run, and returns
// the name of one it could not remove, if any.
//
// The run's own context is cancelled by then, so the cleanup gets a fresh one
// of its own — otherwise it would be cancelled before it started.
func cleanUp(tools *tools, existing []string) string {
	before := make(map[string]bool, len(existing))
	for _, identifier := range existing {
		before[identifier] = true
	}

	var ours []string
	for _, identifier := range tools.docker.ActContainers(context.Background()) {
		if !before[identifier] {
			ours = append(ours, identifier)
		}
	}

	if remaining := tools.docker.RemoveContainers(context.Background(), ours); len(remaining) > 0 {
		return remaining[0]
	}
	return ""
}

// report shows the cause, the action, and the log when it is needed.
func report(env Environment, catalog *i18n.Catalog, diagnosis explain.Diagnosis, verbose bool) {
	fmt.Fprintln(env.Stdout)
	printWrapped(env, catalog.T("run.cause", diagnosis.Cause))
	printWrapped(env, catalog.T("run.action", diagnosis.Action))

	// The excerpt is shown when nothing could be translated, and when the
	// cause is a failing test: the lines are then the answer, not noise.
	if !diagnosis.Known && len(diagnosis.Excerpt) > 0 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, catalog.T("run.excerpt.header"))
		for _, line := range diagnosis.Excerpt {
			fmt.Fprintf(env.Stdout, "  %s\n", strings.TrimRight(line, " "))
		}
	}

	if !verbose {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintln(env.Stdout, catalog.T("run.verbose.hint"))
	}
}

// printStep shows one finished step, with the time act measured for it.
func printStep(env Environment, catalog *i18n.Catalog, step run.Step) {
	mark := markOK
	if step.Status != run.StatusSuccess {
		mark = markProblem
	}
	fmt.Fprintf(env.Stdout, "%s %-*s %s\n", mark, stepColumn, step.Name, humanDuration(catalog, step.Duration))
}

// printWrapped prints a possibly multi-line message, indented as a block.
func printWrapped(env Environment, message string) {
	for _, line := range strings.Split(message, "\n") {
		fmt.Fprintf(env.Stdout, "  %s\n", line)
	}
}

// humanDuration says how long a step took, without decimals: a comma and a
// point are not written the same way in every language, and a beginner does
// not need milliseconds.
func humanDuration(catalog *i18n.Catalog, duration time.Duration) string {
	switch {
	case duration < time.Second:
		return catalog.T("run.duration.instant")
	case duration < time.Minute:
		return catalog.T("run.duration.seconds", int(duration.Seconds()))
	default:
		return catalog.T("run.duration.minutes", int(duration.Minutes()), int(duration.Seconds())%60)
	}
}

// chooseImage settles which image the pipeline runs in, asking once and
// remembering the answer.
//
// This is a deliberate exception to "ask only when detection fails": the
// choice commits more than a gigabyte of download on someone's machine, and
// it cannot be guessed for them.
func chooseImage(env Environment, catalog *i18n.Catalog, override string) string {
	if override != "" {
		return override
	}

	path, pathErr := env.configPath()
	if pathErr == nil {
		if preferences, err := config.Load(path); err == nil && preferences.RunnerImage != "" {
			return preferences.RunnerImage
		}
	}

	if !env.Interactive {
		fmt.Fprintln(env.Stdout, catalog.T("image.question.default_used", run.FaithfulImage))
		return run.FaithfulImage
	}

	fmt.Fprintln(env.Stdout, catalog.T("image.question.intro"))
	fmt.Fprintln(env.Stdout, catalog.T("image.question.option_faithful"))
	fmt.Fprintln(env.Stdout, catalog.T("image.question.option_small"))
	fmt.Fprint(env.Stdout, catalog.T("image.question.prompt"))

	chosen := run.FaithfulImage
	if strings.TrimSpace(readLine(env.Stdin)) == "2" {
		chosen = run.SmallImage
	}

	if pathErr == nil {
		if err := config.Save(path, config.Config{RunnerImage: chosen}); err == nil {
			fmt.Fprintln(env.Stdout, catalog.T("image.question.remembered", path))
		}
	}
	fmt.Fprintln(env.Stdout)

	return chosen
}
