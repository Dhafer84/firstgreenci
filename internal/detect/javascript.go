package detect

import (
	"encoding/json"
	"fmt"
)

// InvalidManifestError reports a manifest file that could not be read as the
// format it claims to be.
type InvalidManifestError struct {
	File string
	Err  error
}

func (e *InvalidManifestError) Error() string {
	return fmt.Sprintf("invalid manifest %s: %v", e.File, e.Err)
}

func (e *InvalidManifestError) Unwrap() error { return e.Err }

// packageJSON holds the few fields of package.json that detection needs.
type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
	Engines struct {
		Node string `json:"node"`
	} `json:"engines"`
}

// detectJavaScript describes a JavaScript project. Unlike Python, the test
// command is not guessed: package.json is where a JavaScript project states
// how it is tested, so a missing "test" script is reported as such.
func detectJavaScript(root string) (*Project, error) {
	raw := readFile(root, "package.json")
	if raw == "" {
		return nil, &NotRecognisedError{Root: root}
	}

	var manifest packageJSON
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return nil, &InvalidManifestError{File: "package.json", Err: err}
	}

	project := &Project{
		Root:           root,
		Language:       JavaScript,
		Manifest:       "package.json",
		RuntimeVersion: nodeVersion(root, manifest),
	}
	project.addEvidence("package.json", "detect.evidence.package_json")

	// The lock file names the tool that installed the dependencies, which is
	// the tool the pipeline must use to reproduce the same install.
	switch {
	case fileExists(root, "pnpm-lock.yaml"):
		project.PackageManager = "pnpm"
		// corepack ships with Node and activates the pinned package manager.
		project.InstallCommand = "corepack enable\npnpm install --frozen-lockfile"
		project.TestCommand = "pnpm test"
		project.addEvidence("pnpm-lock.yaml", "detect.evidence.lock_pnpm")

	case fileExists(root, "yarn.lock"):
		project.PackageManager = "yarn"
		project.InstallCommand = "corepack enable\nyarn install --frozen-lockfile"
		project.TestCommand = "yarn test"
		project.addEvidence("yarn.lock", "detect.evidence.lock_yarn")

	case fileExists(root, "package-lock.json"):
		project.PackageManager = "npm"
		project.InstallCommand = "npm ci"
		project.TestCommand = "npm test"
		project.addEvidence("package-lock.json", "detect.evidence.lock_npm")

	default:
		// No lock file: npm install rather than npm ci, which requires one.
		project.PackageManager = "npm"
		project.InstallCommand = "npm install"
		project.TestCommand = "npm test"
	}

	script, ok := manifest.Scripts["test"]
	if !ok || script == "" {
		return nil, &MissingTestCommandError{Manifest: "package.json"}
	}
	project.TestTool = project.TestCommand
	project.addEvidence("package.json", "detect.evidence.script_test", script)

	return project, nil
}

// nodeVersion reads the version the project pins, and falls back to a recent
// long-term-support release when it pins none.
func nodeVersion(root string, manifest packageJSON) string {
	if pinned := firstVersion(readFile(root, ".nvmrc"), false); pinned != "" {
		return pinned
	}
	if required := firstVersion(manifest.Engines.Node, false); required != "" {
		return required
	}
	return defaultNodeVersion
}
