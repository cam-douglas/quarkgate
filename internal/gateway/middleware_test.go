package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/quarkgate/quarkgate/internal/models"
)

func TestHealthEndpointsBypassAuth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	for _, path := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d", path, rec.Code)
		}
	}
}

func TestScopeMiddlewareDenies(t *testing.T) {
	k := &models.QuarkGateKey{
		ID:     uuid.New(),
		Scopes: json.RawMessage(`["apify"]`),
	}
	route := &models.RouteMatch{Provider: "openrouter", Operation: "chat.completions.create"}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := WithKey(req.Context(), k)
	ctx = WithRoute(ctx, route)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	called := false
	(&ScopeMiddleware{}).Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})).ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not run")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}
