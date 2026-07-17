package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// isolateConfig points LELOIR_CONFIG at a temp file so tests never touch the
// user's real ~/.config/leloir/config.yaml.
func isolateConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("LELOIR_CONFIG", path)
	t.Setenv("LELOIR_SERVER", "")
	t.Setenv("LELOIR_API_KEY", "")
	t.Setenv("LELOIR_CONTEXT", "")
	return path
}

func TestConfigPrecedence(t *testing.T) {
	path := isolateConfig(t)
	if err := os.WriteFile(path, []byte(
		"current-context: file-ctx\ncontexts:\n  - name: file-ctx\n    server: http://from-file:1\n    apiKey: lk_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// File only.
	ctx, err := resolveContext(globals{})
	if err != nil {
		t.Fatalf("resolveContext: %v", err)
	}
	if ctx.Server != "http://from-file:1" || ctx.APIKey != "lk_file" {
		t.Errorf("file ctx = %+v", ctx)
	}

	// Env beats file.
	t.Setenv("LELOIR_SERVER", "http://from-env:2")
	ctx, _ = resolveContext(globals{})
	if ctx.Server != "http://from-env:2" {
		t.Errorf("env must beat file, got %s", ctx.Server)
	}

	// Flags beat env.
	ctx, _ = resolveContext(globals{server: "http://from-flag:3", apiKey: "lk_flag"})
	if ctx.Server != "http://from-flag:3" || ctx.APIKey != "lk_flag" {
		t.Errorf("flags must beat env, got %+v", ctx)
	}

	// Unknown context errors.
	if _, err := resolveContext(globals{context: "ghost"}); err == nil {
		t.Error("unknown context must error")
	}
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("lk_abcdef1234567890"); got != "lk_abcde…7890" {
		t.Errorf("maskKey = %q", got)
	}
	if got := maskKey(""); got != "(none — single-user)" {
		t.Errorf("maskKey empty = %q", got)
	}
}

func TestBearerHeaderSent(t *testing.T) {
	isolateConfig(t)
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"tenant_id":"t","tokens_used":0,"usd_used":0,"investigation_count":0,"budget":{}}`))
	}))
	defer srv.Close()

	if code := run([]string{"--server", srv.URL, "--api-key", "lk_test123", "usage"}); code != 0 {
		t.Fatalf("usage exit = %d", code)
	}
	if gotAuth != "Bearer lk_test123" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

// fakeControlPlane serves POST /alerts and an SSE stream ending in the given outcome.
func fakeControlPlane(t *testing.T, outcome string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"investigation_id":"inv-cli-test","agent":"fake"}`)
	})
	mux.HandleFunc("/api/v1/investigations/inv-cli-test/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: thought\ndata: {\"Type\":\"thought\",\"Payload\":{\"Content\":\"looking\"}}\n\n")
		fmt.Fprint(w, "event: answer\ndata: {\"Type\":\"answer\",\"Payload\":{\"Summary\":\"s\",\"RootCause\":\"rc\",\"Confidence\":0.9}}\n\n")
		fmt.Fprintf(w, "event: complete\ndata: {\"Type\":\"complete\",\"Payload\":{\"Outcome\":%q,\"TotalTokens\":10}}\n\n", outcome)
	})
	return httptest.NewServer(mux)
}

func TestInvestigateFollow_SuccessExitsZero(t *testing.T) {
	isolateConfig(t)
	srv := fakeControlPlane(t, "success")
	defer srv.Close()
	if code := run([]string{"--server", srv.URL, "investigate", "test alert", "--follow"}); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestInvestigateFollow_NonSuccessExitsThree(t *testing.T) {
	isolateConfig(t)
	srv := fakeControlPlane(t, "cancelled")
	defer srv.Close()
	if code := run([]string{"--server", srv.URL, "investigate", "test alert", "--follow"}); code != 3 {
		t.Errorf("exit = %d, want 3 (outcome cancelled)", code)
	}
}

func TestUsageErrors(t *testing.T) {
	isolateConfig(t)
	if code := run([]string{}); code != 2 {
		t.Errorf("sin comando: exit = %d, want 2", code)
	}
	if code := run([]string{"comando-fantasma"}); code != 2 {
		t.Errorf("comando desconocido: exit = %d, want 2", code)
	}
	if code := run([]string{"investigate"}); code != 2 {
		t.Errorf("investigate sin título: exit = %d, want 2", code)
	}
}

func TestAPIErrorExitsOne(t *testing.T) {
	isolateConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	if code := run([]string{"--server", srv.URL, "usage"}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestConfigSetAndUseContext(t *testing.T) {
	isolateConfig(t)
	if code := run([]string{"config", "set-context", "demo", "--server", "http://x:1", "--api-key", "lk_demo"}); code != 0 {
		t.Fatal("set-context failed")
	}
	if code := run([]string{"config", "use-context", "demo"}); code != 0 {
		t.Fatal("use-context failed")
	}
	ctx, err := resolveContext(globals{})
	if err != nil || ctx.Server != "http://x:1" || ctx.APIKey != "lk_demo" {
		t.Errorf("ctx = %+v err=%v", ctx, err)
	}
	// File must be 0600.
	info, _ := os.Stat(configPath())
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLLMCredentialsCommand(t *testing.T) {
	isolateConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/llm-credentials" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"tenant_id":"acme","endpoint":"http://litellm.example.svc:4000",` +
			`"key_secret_name":"acme-llm-key","key_secret_namespace":"tenants",` +
			`"driver":"litellm-operator"}`))
	}))
	defer srv.Close()

	if code := run([]string{"--server", srv.URL, "llm-credentials"}); code != 0 {
		t.Fatalf("llm-credentials exit = %d", code)
	}
	// Alias too.
	if code := run([]string{"--server", srv.URL, "llm-creds"}); code != 0 {
		t.Fatalf("llm-creds exit = %d", code)
	}
}
