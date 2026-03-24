package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clara-phase1/cli"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Strategy != "ar_priority" {
		t.Errorf("expected ar_priority, got %s", cfg.Strategy)
	}
	if len(cfg.ARKinds) == 0 {
		t.Error("expected AR kinds")
	}
	if cfg.NumVerifiers < 1 {
		t.Error("expected at least 1 verifier")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Strategy = "consensus"

	dir := t.TempDir()
	path := filepath.Join(dir, "test_config.json")

	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.Strategy != "consensus" {
		t.Errorf("expected consensus, got %s", loaded.Strategy)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildInteractiveAuto(t *testing.T) {
	p := cli.NewAutoPrompter(nil)
	cfg := BuildInteractive(p)

	if cfg.Strategy != "ar_priority" {
		t.Errorf("expected default strategy, got %s", cfg.Strategy)
	}
	if len(cfg.ARKinds) == 0 {
		t.Error("expected AR kinds from defaults")
	}
}

func TestBuildInteractiveOverrides(t *testing.T) {
	overrides := map[string]string{
		"strategy":      "consensus",
		"num_verifiers": "5",
	}
	p := cli.NewAutoPrompter(overrides)
	cfg := BuildInteractive(p)

	if cfg.Strategy != "consensus" {
		t.Errorf("expected consensus, got %s", cfg.Strategy)
	}
	if cfg.NumVerifiers != 5 {
		t.Errorf("expected 5 verifiers, got %d", cfg.NumVerifiers)
	}
}

func TestConfigSaveDoesNotExist(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.SaveToFile("/nonexistent/dir/config.json")
	if err == nil {
		os.Remove("/nonexistent/dir/config.json")
	}
}
