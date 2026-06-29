package registry

import (
	"net/http"
	"testing"
)

func TestCompatPathsMatchRoute(t *testing.T) {
	reg, err := Load(driversRoot(t), "node")
	if err != nil {
		t.Fatal(err)
	}
	for id, m := range reg.AllManifests() {
		for _, op := range m.Operations {
			for _, cp := range op.CompatPaths {
				match, err := reg.MatchRoute(op.Method, cp)
				if err != nil {
					t.Fatalf("%s compat %s %s: %v", id, op.Method, cp, err)
				}
				if match.Provider != id {
					t.Fatalf("%s compat %s: got provider %s", id, cp, match.Provider)
				}
				if match.Operation != op.ID {
					t.Fatalf("%s compat %s: got operation %s want %s", id, cp, match.Operation, op.ID)
				}
			}
			providerPath := "/v1/providers/" + id + op.Path
			match, err := reg.MatchRoute(op.Method, providerPath)
			if err != nil {
				t.Fatalf("%s provider path %s: %v", id, providerPath, err)
			}
			if match.Provider != id || match.Operation != op.ID {
				t.Fatalf("%s provider path mismatch", id)
			}
		}
	}
}

func TestOpenAICompatRoute(t *testing.T) {
	reg, err := Load(driversRoot(t), "node")
	if err != nil {
		t.Fatal(err)
	}
	m, err := reg.MatchRoute(http.MethodPost, "/v1/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "openrouter" {
		t.Fatalf("got %s", m.Provider)
	}
}
