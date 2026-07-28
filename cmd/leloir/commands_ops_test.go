package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOpsCommands runs each new read-surface command against a mock control plane
// and asserts it drives the right endpoint and exits 0. Guards the CLI↔API contract
// (the CLI is the reference client) so a renamed/removed endpoint is caught here.
func TestOpsCommands(t *testing.T) {
	isolateConfig(t)
	var bundleFramework string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/metrics/summary", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tenant":"acme","investigations":3,"tokens_used":100,"usd_used":0.03,"hitl_pending":1,` +
			`"agents":{"total":2,"off_radar":1,"drift":0},"reliability":{"investigated":3,"resolved":2,"mean_time_to_resolve_sec":120}}`))
	})
	mux.HandleFunc("/api/v1/metrics/trends", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"series":[{"key":"spend_usd","title":"Spend","unit":"USD"}]}`))
	})
	mux.HandleFunc("/api/v1/roi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"window_days":7,"investigated":3,"resolved":2,"resolve_rate":0.66,"total_usd":0.03,` +
			`"total_tokens":100,"usd_per_resolved":0.015,"mean_time_to_resolve_sec":120}`))
	})
	mux.HandleFunc("/api/v1/license", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tier":"oss","licensed":false,"enforced":false,"features":[{"name":"a","available":true}]}`))
	})
	mux.HandleFunc("/api/v1/capabilities", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"source":"graph","engine_tier":"oss","registered_agents":2,"count":23}`))
	})
	mux.HandleFunc("/api/v1/compliance/evidence", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tenant":"acme","controls":[{"framework":"SOC2","control":"CC7.2","leloir_capability":"alert-routing",` +
			`"capability_test_status":"e2e-happy","evidence_count":3,"tamper_evident":true}]}`))
	})
	mux.HandleFunc("/api/v1/compliance/bundle", func(w http.ResponseWriter, r *http.Request) {
		bundleFramework = r.URL.Query().Get("framework")
		w.Write([]byte(`{"tenant":"acme","hash_chain":{"enabled":true,"verified":true,"events":42},"controls":[],"audit_events":[]}`))
	})
	mux.HandleFunc("/api/v1/quarantine", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"quarantined":[]}`))
	})
	mux.HandleFunc("/api/v1/approvals", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})
	mux.HandleFunc("/api/v1/usage/projection", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tenant":"acme","days_elapsed":10,"days_in_month":31,"daily_burn_usd":0.5,` +
			`"projected_usd":15.5,"budget_usd":10,"projected_pct_of_budget":155,"status":"will_exceed",` +
			`"method":"extrapolación lineal — NO es una predicción"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := [][]string{
		{"metrics"}, // defaults to summary
		{"metrics", "summary"},
		{"metrics", "trends"}, // catalog
		{"roi", "--days", "7"},
		{"license"},
		{"capabilities"},
		{"caps"}, // alias
		{"compliance", "evidence", "--framework", "SOC2"},
		{"compliance", "bundle", "--framework", "EU-DORA"},
		{"quarantine"},
		{"approvals"},
		{"usage", "projection"},          // burn-rate projection
		{"--json", "metrics", "summary"}, // raw-JSON path
	}
	for _, args := range cases {
		full := append([]string{"--server", srv.URL}, args...)
		if code := run(full); code != 0 {
			t.Errorf("%v: exit = %d, want 0", args, code)
		}
	}
	if bundleFramework != "EU-DORA" {
		t.Errorf("compliance bundle should pass ?framework=EU-DORA, got %q", bundleFramework)
	}
}

// TestOpsCommandsUsageErrors: bad subcommands exit 2 (usage error).
func TestOpsCommandsUsageErrors(t *testing.T) {
	isolateConfig(t)
	for _, args := range [][]string{
		{"metrics", "nope"},
		{"compliance"},
		{"compliance", "nope"},
	} {
		if code := run(append([]string{"--server", "http://unused"}, args...)); code != 2 {
			t.Errorf("%v: exit = %d, want 2", args, code)
		}
	}
}
