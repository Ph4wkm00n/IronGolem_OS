package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeProviderRuntime implements providerRuntimeClient so tests can drive
// the handler without spinning up runtimed. Mirrors what runtimed would
// emit on the wire so any drift between the real and fake catches at
// compile time (interface conformance) + at runtime (shape comparison).
type fakeProviderRuntime struct {
	active   string
	profiles []json.RawMessage
	err      error
}

func (f fakeProviderRuntime) ListProvidersRaw(_ context.Context) (string, []json.RawMessage, error) {
	if f.err != nil {
		return "", nil, f.err
	}
	return f.active, f.profiles, nil
}

func TestProviderHandler_ListProviders_OK(t *testing.T) {
	prof := []json.RawMessage{
		json.RawMessage(`{"name":"mock","display_name":"Mock"}`),
		json.RawMessage(`{"name":"anthropic","display_name":"Anthropic Claude"}`),
		json.RawMessage(`{"name":"openai","display_name":"OpenAI"}`),
	}
	h := newProviderHandlerWithRuntime(slog.Default(), fakeProviderRuntime{
		active:   "anthropic",
		profiles: prof,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	h.ListProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v; raw=%s", err, rec.Body.String())
	}
	if body["active"] != "anthropic" {
		t.Fatalf("active = %v, want anthropic", body["active"])
	}
	profsAny, ok := body["profiles"].([]any)
	if !ok || len(profsAny) != 3 {
		t.Fatalf("profiles missing or wrong length: %v", body["profiles"])
	}
}

func TestProviderHandler_ListProviders_RuntimeError(t *testing.T) {
	h := newProviderHandlerWithRuntime(slog.Default(), fakeProviderRuntime{
		err: errFake("runtimed offline"),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	h.ListProviders(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "runtime_unavailable") {
		t.Fatalf("body missing runtime_unavailable: %s", body)
	}
}

func TestProviderHandler_ListProviders_SkipsUndecodableProfile(t *testing.T) {
	// First profile is intentionally malformed JSON. The handler must
	// drop it and keep the rest of the list — partial output is better
	// than a 502 when one provider has a buggy serializer.
	prof := []json.RawMessage{
		json.RawMessage(`{not json`),
		json.RawMessage(`{"name":"anthropic"}`),
	}
	h := newProviderHandlerWithRuntime(slog.Default(), fakeProviderRuntime{
		active:   "anthropic",
		profiles: prof,
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	h.ListProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	profs := body["profiles"].([]any)
	if len(profs) != 1 {
		t.Fatalf("profiles length = %d, want 1 (bad profile dropped)", len(profs))
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
