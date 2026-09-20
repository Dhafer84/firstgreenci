package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Dhafer84/firstgreenci/internal/config"
	"github.com/Dhafer84/firstgreenci/internal/explain"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
	"github.com/Dhafer84/firstgreenci/internal/run"
)

// Marks shown in front of each check. They are symbols, not words: a tick
// needs no translation, and the sentence next to it carries the meaning.
const (
	markOK      = "✓"
	markProblem = "✗"
	markNote    = "·"
	// A warning is not a failure: the command succeeds, but something will
	// go wrong later if nothing is done.
	markWarning = "⚠"
)

// labelWidth lines the statuses up under one another.
const labelWidth = 8

// runDoctor reports what is ready and what is missing before a pipeline can
// run locally, with the exact steps for each missing piece.
//
// It answers the part of the MVP about "checking Docker and guiding its
// installation", and what the interviews showed: 4 people out of 7 were
// blocked before writing anything, on installing or choosing a tool.
func runDoctor(env Environment, args []string) int {
	flags := newFlagSet("doctor")
	lang := flags.String("lang", "", "")
	parseErr := flags.Parse(args)

	catalog, code := load(env, *lang)
	if catalog == nil {
		return code
	}
	if parseErr != nil {
		return reportUsageError(env, catalog, parseErr)
	}

	ready := true
	ctx := env.context()

	// Docker: installed, and its daemon answering, are two different
	// questions with two different answers.
	docker, dockerErr := run.FindDocker()
	daemonRunning := false

	switch {
	case dockerErr != nil:
		ready = false
		printCheck(env, catalog.T("doctor.docker.label"), markProblem, catalog.T("doctor.docker.not_installed"))
		printGuide(env, explain.DockerInstallGuide(catalog, env.system()))

	default:
		if err := docker.CheckDaemon(ctx); err != nil {
			ready = false
			printCheck(env, catalog.T("doctor.docker.label"), markProblem, catalog.T("doctor.docker.not_running"))
			printGuide(env, explain.DockerStartGuide(catalog, env.system()))
		} else {
			daemonRunning = true
			printCheck(env, catalog.T("doctor.docker.label"), markOK, catalog.T("doctor.docker.ok"))
		}
	}

	if _, err := run.FindAct(); err != nil {
		ready = false
		printCheck(env, catalog.T("doctor.act.label"), markProblem, catalog.T("doctor.act.not_installed"))
		printGuide(env, explain.ActInstallGuide(catalog, env.system()))
	} else {
		printCheck(env, catalog.T("doctor.act.label"), markOK, catalog.T("doctor.act.ok"))
	}

	printImageCheck(env, catalog, docker, daemonRunning, ctx)

	fmt.Fprintln(env.Stdout)
	if !ready {
		fmt.Fprintln(env.Stdout, catalog.T("doctor.not_ready"))
		return exitFailure
	}
	fmt.Fprintln(env.Stdout, catalog.T("doctor.ready"))
	return exitSuccess
}

// printImageCheck reports the container image. A missing image is a note, not
// a problem: it downloads by itself on the first run.
func printImageCheck(env Environment, catalog *i18n.Catalog, docker *run.Docker, daemonRunning bool, ctx context.Context) {
	image := run.DefaultImage
	if path, err := env.configPath(); err == nil {
		if preferences, err := config.Load(path); err == nil && preferences.RunnerImage != "" {
			image = preferences.RunnerImage
		}
	}

	label := catalog.T("doctor.image.label")
	if !daemonRunning {
		printCheck(env, label, markNote, catalog.T("doctor.image.unknown"))
		return
	}
	if docker.HasImage(ctx, image) {
		printCheck(env, label, markOK, catalog.T("doctor.image.cached", image))
		return
	}
	printCheck(env, label, markNote, catalog.T("doctor.image.missing", image))
}

func printCheck(env Environment, label, mark, status string) {
	fmt.Fprintf(env.Stdout, "%-*s %s %s\n", labelWidth, label, mark, status)
}

// printGuide shows a multi-line instruction, indented under the check it
// belongs to.
func printGuide(env Environment, guide string) {
	fmt.Fprintln(env.Stdout)
	for _, line := range strings.Split(guide, "\n") {
		fmt.Fprintf(env.Stdout, "  %s\n", line)
	}
	fmt.Fprintln(env.Stdout)
}
