//go:build e2e

package e2e_test

import (
	"os"
	"testing"
)

func TestLiveOpenRouterRequiresEnv(t *testing.T) {
	if os.Getenv("QUARKGATE_KEY") == "" {
		t.Skip("QUARKGATE_KEY not set — manual live E2E only")
	}
	if os.Getenv("QUARKGATE_URL") == "" {
		t.Skip("QUARKGATE_URL not set")
	}
	// Live HTTP calls documented in examples/swarm-minimal/README.md
}
