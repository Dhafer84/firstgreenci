package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectPython describes a Python project: how its dependencies are declared,
// which tool installs them, and which command runs its tests.
func detectPython(root string) (*Project, error) {
	version, source := pythonVersion(root)
	project := &Project{
		Root:                 root,
		Language:             Python,
		RuntimeVersion:       version,
		RuntimeVersionSource: source,
	}

	pyproject := readFile(root, "pyproject.toml")

	// The order matters: a project can carry several dependency files, and
	// the most specific tool wins.
	switch {
	case pyproject != "" && strings.Contains(pyproject, "[tool.poetry]"):
		project.PackageManager = "poetry"
		project.Manifest = "pyproject.toml"
		project.InstallCommand = "python -m pip install poetry\npoetry install"
		project.addEvidence("pyproject.toml", "detect.evidence.pyproject_poetry")

	case fileExists(root, "Pipfile"):
		project.PackageManager = "pipenv"
		project.Manifest = "Pipfile"
		project.InstallCommand = "python -m pip install pipenv\npipenv install --dev"
		project.addEvidence("Pipfile", "detect.evidence.pipfile")

	case fileExists(root, "requirements.txt"):
		project.PackageManager = "pip"
		project.Manifest = "requirements.txt"
		project.InstallCommand = "python -m pip install --upgrade pip\npython -m pip install -r requirements.txt"
		project.addEvidence("requirements.txt", "detect.evidence.requirements_txt")

		// Whatever the detection reads, the pipeline must install. Reading
		// requirements-dev.txt to conclude that a project uses pytest, then
		// installing only requirements.txt, produced a workflow that failed
		// on "pytest: command not found".
		if fileExists(root, devRequirements) {
			project.InstallCommand += "\npython -m pip install -r " + devRequirements
			project.addEvidence(devRequirements, "detect.evidence.requirements_dev")
		}

	case pyproject != "":
		project.PackageManager = "pip"
		project.Manifest = "pyproject.toml"
		project.InstallCommand = "python -m pip install --upgrade pip\npython -m pip install ."
		project.addEvidence("pyproject.toml", "detect.evidence.pyproject")

	case fileExists(root, "setup.py"):
		project.PackageManager = "pip"
		project.Manifest = "setup.py"
		project.InstallCommand = "python -m pip install --upgrade pip\npython -m pip install ."
		project.addEvidence("setup.py", "detect.evidence.setup_py")

	default:
		return nil, &NotRecognisedError{Root: root}
	}

	project.TestTool, project.TestCommand = pythonTestCommand(root, project)
	project.addTestEvidence(root)
	project.warnAboutUncollectableTests(root)
	project.noteMissingRuntimeVersion("detect.note.no_runtime_version.python")

	return project, nil
}

// pythonTestCommand picks the testing tool. pytest is only used when the
// project declares it as a dependency: running a tool that the pipeline never
// installs would turn the very first run red.
func pythonTestCommand(root string, project *Project) (tool, command string) {
	prefix := ""
	switch project.PackageManager {
	case "poetry":
		prefix = "poetry run "
	case "pipenv":
		prefix = "pipenv run "
	}

	if pytestDeclaredIn(root) != "" {
		return "pytest", prefix + "pytest"
	}
	return "unittest", prefix + "python -m unittest discover"
}

// devRequirements is the one file for development-only dependencies the
// tool knows. Other names exist in the wild (dev-requirements.txt,
// requirements-test.txt); none has been met yet, and a name added from
// memory is a guess.
const devRequirements = "requirements-dev.txt"

// pytestDeclaredIn returns the file that declares pytest, or an empty string
// when none does. Returning the file rather than a yes or no lets the report
// name where it was seen: saying "requirements.txt: pytest is listed there"
// when it sits in requirements-dev.txt sends the reader to the wrong place.
func pytestDeclaredIn(root string) string {
	for _, name := range []string{"requirements.txt", devRequirements, "pyproject.toml", "Pipfile", "setup.py", "setup.cfg", "tox.ini"} {
		if strings.Contains(readFile(root, name), "pytest") {
			return name
		}
	}
	return ""
}

// pythonVersion reads the version the project pins, and falls back to a
// recent one when it pins none.
//
// It returns the file the version came from, empty on the fallback. Saying
// "the same version as on your computer" about a number nobody declared is a
// claim the tool cannot make.
func pythonVersion(root string) (version, source string) {
	if pinned := firstVersion(readFile(root, ".python-version"), true); pinned != "" {
		return pinned, ".python-version"
	}

	for _, line := range strings.Split(readFile(root, "pyproject.toml"), "\n") {
		trimmed := strings.TrimSpace(line)
		// PEP 621 writes requires-python, Poetry writes python = "^3.11".
		if !strings.HasPrefix(trimmed, "requires-python") && !strings.HasPrefix(trimmed, "python =") {
			continue
		}
		if required := firstVersion(trimmed, true); required != "" {
			return required, "pyproject.toml"
		}
	}

	return defaultPythonVersion, ""
}

// addTestEvidence records where the tests were found, when they can be seen.
func (p *Project) addTestEvidence(root string) {
	if declaring := pytestDeclaredIn(root); declaring != "" {
		p.addEvidence(declaring, "detect.evidence.pytest_dependency")
	}

	if dirExists(root, "tests") {
		p.addEvidence("tests", "detect.evidence.tests_dir")
		return
	}

	matches, err := filepath.Glob(filepath.Join(root, "test_*.py"))
	if err == nil && len(matches) > 0 {
		p.addEvidence(filepath.Base(root), "detect.evidence.test_files")
	}
}

// Counting mentions of a name is not counting declarations. A file whose
// docstring says "written without unittest.TestCase" holds no TestCase at
// all, and a plain substring search reads it as one — which it did, on the
// very fixture written to exercise this code. Both patterns match
// declarations, anchored to the start of a line.
var (
	testFunctionPattern = regexp.MustCompile(`(?m)^[ \t]*def[ \t]+test_\w*[ \t]*\(`)
	testCasePattern     = regexp.MustCompile(`(?m)^[ \t]*class[ \t]+\w+[ \t]*\([^)]*TestCase[^)]*\)`)
)

// testShape is what the test files themselves show, as opposed to what the
// dependency files declare.
type testShape struct {
	files     int
	functions int
	testCases int
}

// inspectTests counts what matters in the test files: how many there are,
// how many test functions they hold, and how many unittest.TestCase classes.
//
// The detection reads dependency declarations to decide which command to
// generate. It reads the tests themselves only to advise, never to decide.
func inspectTests(root string) testShape {
	paths, _ := filepath.Glob(filepath.Join(root, "tests", "test_*.py"))
	atRoot, _ := filepath.Glob(filepath.Join(root, "test_*.py"))
	paths = append(paths, atRoot...)

	shape := testShape{files: len(paths)}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		shape.functions += len(testFunctionPattern.FindAllIndex(content, -1))
		shape.testCases += len(testCasePattern.FindAllIndex(content, -1))
	}
	return shape
}

// warnAboutUncollectableTests says, before anything runs, that the generated
// pipeline will find no test at all.
//
// unittest only collects classes deriving from unittest.TestCase. A project
// whose tests are plain test_ functions, with no test tool declared, gets a
// pipeline that runs to the end and collects nothing — a red result, after
// minutes of waiting, for someone who did nothing wrong. It was met on a
// real project: 19 files, 406 functions, no TestCase.
//
// The condition is deliberately narrow: no existing sample project triggers
// it, including the one that uses unittest properly.
func (p *Project) warnAboutUncollectableTests(root string) {
	if pytestDeclaredIn(root) != "" {
		return
	}

	shape := inspectTests(root)
	if shape.files == 0 || shape.functions == 0 || shape.testCases > 0 {
		return
	}

	p.Warnings = append(p.Warnings, Notice{
		MessageKey: "detect.warning.no_test_tool",
		Args:       []any{shape.files, shape.functions},
	})
}

// noteMissingRuntimeVersion says, quietly, that the version in the pipeline
// was chosen rather than read.
//
// It deliberately does not show the version the machine runs. On the project
// that exposed this defect, python3 reports 3.14 while the project itself
// runs 3.12 inside its .venv — announcing the first would repeat the very
// mistake being fixed: claiming to know what cannot be known from here.
func (p *Project) noteMissingRuntimeVersion(messageKey string) {
	if p.RuntimeVersionSource != "" {
		return
	}
	p.Notes = append(p.Notes, Notice{MessageKey: messageKey, Args: []any{p.RuntimeVersion}})
}

// addEvidence appends one observed fact to the project.
func (p *Project) addEvidence(file, messageKey string, args ...any) {
	p.Evidence = append(p.Evidence, Evidence{File: file, MessageKey: messageKey, Args: args})
}
