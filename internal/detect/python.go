package detect

import (
	"path/filepath"
	"strings"
)

// detectPython describes a Python project: how its dependencies are declared,
// which tool installs them, and which command runs its tests.
func detectPython(root string) (*Project, error) {
	project := &Project{
		Root:           root,
		Language:       Python,
		RuntimeVersion: pythonVersion(root),
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

	if declaresPytest(root) {
		return "pytest", prefix + "pytest"
	}
	return "unittest", prefix + "python -m unittest discover"
}

// declaresPytest reports whether pytest appears in any of the files where a
// Python project declares its dependencies.
func declaresPytest(root string) bool {
	for _, name := range []string{"requirements.txt", "requirements-dev.txt", "pyproject.toml", "Pipfile", "setup.py", "setup.cfg", "tox.ini"} {
		if strings.Contains(readFile(root, name), "pytest") {
			return true
		}
	}
	return false
}

// pythonVersion reads the version the project pins, and falls back to a
// recent one when it pins none.
func pythonVersion(root string) string {
	if pinned := firstVersion(readFile(root, ".python-version"), true); pinned != "" {
		return pinned
	}

	for _, line := range strings.Split(readFile(root, "pyproject.toml"), "\n") {
		trimmed := strings.TrimSpace(line)
		// PEP 621 writes requires-python, Poetry writes python = "^3.11".
		if !strings.HasPrefix(trimmed, "requires-python") && !strings.HasPrefix(trimmed, "python =") {
			continue
		}
		if required := firstVersion(trimmed, true); required != "" {
			return required
		}
	}

	return defaultPythonVersion
}

// addTestEvidence records where the tests were found, when they can be seen.
func (p *Project) addTestEvidence(root string) {
	if declaresPytest(root) {
		p.addEvidence(p.Manifest, "detect.evidence.pytest_dependency")
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

// addEvidence appends one observed fact to the project.
func (p *Project) addEvidence(file, messageKey string, args ...any) {
	p.Evidence = append(p.Evidence, Evidence{File: file, MessageKey: messageKey, Args: args})
}
