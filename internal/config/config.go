// Package config stores the few preferences that must outlive a run.
//
// There is exactly one today: the container image used to run pipelines
// locally. It is remembered because choosing it means downloading more than a
// gigabyte, and nobody should be asked that twice.
//
// Nothing secret is ever written here. The file is plain JSON, in the usual
// configuration folder of the system, and a user can read or delete it.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the content of the preferences file.
type Config struct {
	// RunnerImage is the container image act runs the pipeline in. An empty
	// value means the question has not been answered yet.
	RunnerImage string `json:"runner_image,omitempty"`
}

// Path returns the preferences file of the current user. os.UserConfigDir
// gives the right folder on each system: ~/.config on Linux,
// ~/Library/Application Support on macOS, %AppData% on Windows.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the configuration folder: %w", err)
	}
	return filepath.Join(dir, "firstgreenci", "config.json"), nil
}

// Load reads the preferences stored at path.
//
// A missing file is not a failure: it is what a first run looks like, and it
// yields empty preferences. A damaged file is not a failure either — the tool
// must not refuse to work because of its own cache — but it is reported, so
// that the caller can mention it instead of silently forgetting a choice.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read preferences %s: %w", path, err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return Config{}, fmt.Errorf("parse preferences %s: %w", path, err)
	}
	return loaded, nil
}

// Save writes the preferences to path, creating the folder if needed.
func Save(path string, config Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create the configuration folder %s: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write preferences %s: %w", path, err)
	}
	return nil
}
