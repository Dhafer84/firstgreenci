package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/config"
)

func TestLoadMissingFileIsAFirstRun(t *testing.T) {
	got, err := config.Load(filepath.Join(t.TempDir(), "absent", "config.json"))
	if err != nil {
		t.Fatalf("Load on a missing file returned an error: %v", err)
	}
	if got.RunnerImage != "" {
		t.Errorf("RunnerImage = %q, want empty", got.RunnerImage)
	}
}

func TestSaveThenLoad(t *testing.T) {
	// A nested folder checks that Save creates what it needs.
	path := filepath.Join(t.TempDir(), "firstgreenci", "config.json")
	want := config.Config{RunnerImage: "catthehacker/ubuntu:act-latest"}

	if err := config.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RunnerImage != want.RunnerImage {
		t.Errorf("RunnerImage = %q, want %q", got.RunnerImage, want.RunnerImage)
	}
}

func TestLoadDamagedFileIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := config.Load(path); err == nil {
		t.Fatal("Load accepted a damaged file, want an error")
	}
}

func TestPathIsUnderTheConfigurationFolder(t *testing.T) {
	path, err := config.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if filepath.Base(path) != "config.json" {
		t.Errorf("the file is named %q, want config.json", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != "firstgreenci" {
		t.Errorf("the folder is %q, want firstgreenci", filepath.Base(filepath.Dir(path)))
	}
}
