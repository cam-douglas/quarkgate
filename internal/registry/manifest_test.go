package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func driversRoot(t *testing.T) string {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "..", "..", "drivers")
	if _, err := os.Stat(p); err != nil {
		p = "drivers"
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("drivers directory not found")
	}
	return p
}

func TestManifestsValid(t *testing.T) {
	driversPath := driversRoot(t)
	entries, err := os.ReadDir(driversPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") || e.Name() == "fixtures" || e.Name() == "sdk" {
			continue
		}
		manifestPath := filepath.Join(driversPath, e.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s unmarshal: %v", e.Name(), err)
		}
		if m.ID == "" {
			t.Fatalf("%s: missing id", e.Name())
		}
		if m.ID != e.Name() {
			t.Fatalf("%s: id %s != folder", e.Name(), m.ID)
		}
		if m.Version == "" {
			t.Fatalf("%s: missing version", e.Name())
		}
		if m.Category == "" {
			t.Fatalf("%s: missing category", e.Name())
		}
		if len(m.Operations) == 0 {
			t.Fatalf("%s: no operations", e.Name())
		}
		for _, op := range m.Operations {
			if op.ID == "" || op.Method == "" || op.Path == "" {
				t.Fatalf("%s: invalid operation", e.Name())
			}
		}
		driverJS := filepath.Join(driversPath, e.Name(), "driver.js")
		driverPY := filepath.Join(driversPath, e.Name(), "driver.py")
		if _, err := os.Stat(driverJS); err != nil {
			if _, err2 := os.Stat(driverPY); err2 != nil {
				t.Fatalf("%s: no driver.js or driver.py", e.Name())
			}
		}
	}
}

func TestLoadRegistry(t *testing.T) {
	reg, err := Load(driversRoot(t), "node")
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.AllManifests()) < 4 {
		t.Fatalf("expected 4+ manifests, got %d", len(reg.AllManifests()))
	}
}
