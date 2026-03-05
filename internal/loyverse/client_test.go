package loyverse_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aria/internal/loyverse"
)

// newTestClient crea un cliente apuntando a un servidor de test.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *loyverse.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := loyverse.NewClient(srv.Client(), "test-token", loyverse.WithBaseURL(srv.URL))
	return srv, client
}

// mustJSON serializa v a JSON o falla el test.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return b
}

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	client := loyverse.NewClient(nil, "token")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestBuildRequest_SendsAuthHeader(t *testing.T) {
	var capturedAuth string
	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, loyverse.ReceiptsResponse{}))
	})

	client.GetCategories(t.Context())

	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", capturedAuth, "Bearer test-token")
	}
}
