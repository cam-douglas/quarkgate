package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifests(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// tests run from package dir; find repo drivers
	driversPath := filepath.Join(root, "..", "..", "drivers")
	if _, err := os.Stat(driversPath); err != nil {
		driversPath = "drivers"
	}
	reg, err := Load(driversPath, "node")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.GetManifest("openrouter"); !ok {
		t.Fatal("openrouter manifest missing")
	}
}

func TestMatchOpenAICompat(t *testing.T) {
	root := "drivers"
	if _, err := os.Stat(root); err != nil {
		t.Skip("drivers not in cwd")
	}
	reg, err := Load(root, "node")
	if err != nil {
		t.Fatal(err)
	}
	m, err := reg.MatchRoute("POST", "/v1/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "openrouter" {
		t.Fatalf("got %s", m.Provider)
	}
}
