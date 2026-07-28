package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// This file adds the read-surface commands that mirror the newer /api/v1 endpoints
// (the CLI is the reference client, a pure HTTP client of the API). Flat responses
// get a human summary; rich/nested ones are printed as indented JSON. All honor
// --json (raw body) via c.emitJSON.

// ─── leloir metrics ───────────────────────────────────────────────────────────
// GET /metrics/summary (headline) + /metrics/trends[/{metric}] (time series).

func cmdMetrics(c *client, args []string) int {
	sub := "summary"
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub, rest = args[0], args[1:]
	}
	switch sub {
	case "summary":
		return metricsSummary(c)
	case "trends":
		return metricsTrends(c, rest)
	default:
		fmt.Fprintln(os.Stderr, "uso: leloir metrics [summary | trends [<metric>]]")
		return 2
	}
}

func metricsSummary(c *client) int {
	var s struct {
		Tenant         string  `json:"tenant"`
		Investigations int64   `json:"investigations"`
		TokensUsed     int64   `json:"tokens_used"`
		USDUsed        float64 `json:"usd_used"`
		HITLPending    int     `json:"hitl_pending"`
		Agents         struct {
			Total    int `json:"total"`
			OffRadar int `json:"off_radar"`
			Drift    int `json:"drift"`
		} `json:"agents"`
		Reliability struct {
			Investigated         int     `json:"investigated"`
			Resolved             int     `json:"resolved"`
			MeanTimeToResolveSec float64 `json:"mean_time_to_resolve_sec"`
		} `json:"reliability"`
	}
	body, err := c.getJSON("/metrics/summary", &s)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("tenant:          %s\n", s.Tenant)
	fmt.Printf("investigaciones: %d\n", s.Investigations)
	fmt.Printf("gasto:           $%.4f · %d tokens\n", s.USDUsed, s.TokensUsed)
	fmt.Printf("HITL pendientes: %d\n", s.HITLPending)
	fmt.Printf("agentes:         %d (off-radar %d · drift %d)\n", s.Agents.Total, s.Agents.OffRadar, s.Agents.Drift)
	mttr := ""
	if s.Reliability.MeanTimeToResolveSec > 0 {
		mttr = fmt.Sprintf(" · MTTR %.0fs", s.Reliability.MeanTimeToResolveSec)
	}
	fmt.Printf("fiabilidad:      %d/%d resueltas%s\n", s.Reliability.Resolved, s.Reliability.Investigated, mttr)
	return 0
}

func metricsTrends(c *client, args []string) int {
	// No metric → the catalog (what series exist). With a metric → its time series.
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		body, err := c.getJSON("/metrics/trends", nil)
		if err != nil {
			return fail(err)
		}
		if c.emitJSON(body) {
			return 0
		}
		printPrettyJSON(body)
		return 0
	}
	metric := args[0]
	fs := newFlagSet("metrics trends")
	window := fs.String("window", "24h", "ventana (ej. 24h, 7d)")
	step := fs.String("step", "", "resolución (ej. 1h; vacío = auto)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	q := url.Values{}
	q.Set("window", *window)
	if *step != "" {
		q.Set("step", *step)
	}
	body, status, err := c.do(http.MethodGet, "/metrics/trends/"+url.PathEscape(metric)+"?"+q.Encode(), nil)
	if err != nil {
		return fail(err)
	}
	if status >= 400 {
		return fail(apiError(status, body))
	}
	if c.emitJSON(body) {
		return 0
	}
	printPrettyJSON(body)
	return 0
}

// ─── leloir roi ───────────────────────────────────────────────────────────────

func cmdROI(c *client, args []string) int {
	fs := newFlagSet("roi")
	days := fs.Int("days", 30, "ventana en días")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var r struct {
		WindowDays           int     `json:"window_days"`
		Investigated         int     `json:"investigated"`
		Resolved             int     `json:"resolved"`
		ResolveRate          float64 `json:"resolve_rate"`
		TotalUSD             float64 `json:"total_usd"`
		TotalTokens          int64   `json:"total_tokens"`
		USDPerResolved       float64 `json:"usd_per_resolved"`
		MeanTimeToResolveSec float64 `json:"mean_time_to_resolve_sec"`
	}
	body, err := c.getJSON(fmt.Sprintf("/roi?days=%d", *days), &r)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("ventana:            %d días\n", r.WindowDays)
	fmt.Printf("investigadas:       %d\n", r.Investigated)
	fmt.Printf("resueltas:          %d (%.0f%%)\n", r.Resolved, r.ResolveRate*100)
	fmt.Printf("gasto:              $%.4f · %d tokens\n", r.TotalUSD, r.TotalTokens)
	if r.USDPerResolved > 0 {
		fmt.Printf("USD por resuelta:   $%.4f\n", r.USDPerResolved)
	}
	if r.MeanTimeToResolveSec > 0 {
		fmt.Printf("MTTR:               %.0fs\n", r.MeanTimeToResolveSec)
	}
	return 0
}

// ─── leloir license ───────────────────────────────────────────────────────────

func cmdLicense(c *client) int {
	var s struct {
		Tier     string `json:"tier"`
		Licensed bool   `json:"licensed"`
		Enforced bool   `json:"enforced"`
		Features []struct {
			Name      string `json:"name"`
			Available bool   `json:"available"`
		} `json:"features"`
	}
	body, err := c.getJSON("/license", &s)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("tier:      %s\n", s.Tier)
	fmt.Printf("con key:   %v\n", s.Licensed)
	fmt.Printf("enforced:  %v\n", s.Enforced)
	avail := 0
	for _, f := range s.Features {
		if f.Available {
			avail++
		}
	}
	fmt.Printf("features:  %d/%d disponibles (el motor es 100%% libre)\n", avail, len(s.Features))
	return 0
}

// ─── leloir capabilities ──────────────────────────────────────────────────────

func cmdCapabilities(c *client) int {
	var caps struct {
		Source           string `json:"source"`
		EngineTier       string `json:"engine_tier"`
		RegisteredAgents int    `json:"registered_agents"`
		Count            int    `json:"count"`
	}
	body, err := c.getJSON("/capabilities", &caps)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("fuente:            %s\n", caps.Source)
	fmt.Printf("tier del motor:    %s\n", caps.EngineTier)
	fmt.Printf("agentes:           %d\n", caps.RegisteredAgents)
	fmt.Printf("capabilities:      %d (usá --json para el detalle del grafo ∩ cluster)\n", caps.Count)
	return 0
}

// ─── leloir compliance ────────────────────────────────────────────────────────
// evidence: audit WORM mapeado a controles. bundle: paquete descargable para auditor.

func cmdCompliance(c *client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: leloir compliance [evidence | bundle] [--framework NIST-AI-RMF|SOC2|EU-DORA|GDPR|ISO-27001|...]")
		return 2
	}
	sub, rest := args[0], args[1:]
	fs := newFlagSet("compliance " + sub)
	framework := fs.String("framework", "", "filtrar por marco (ej. EU-DORA)")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	q := ""
	if *framework != "" {
		q = "?framework=" + url.QueryEscape(*framework)
	}
	switch sub {
	case "evidence":
		return complianceEvidence(c, q)
	case "bundle":
		return complianceBundle(c, q)
	default:
		fmt.Fprintln(os.Stderr, "uso: leloir compliance [evidence | bundle] [--framework ...]")
		return 2
	}
}

func complianceEvidence(c *client, q string) int {
	var e struct {
		Tenant   string `json:"tenant"`
		Controls []struct {
			Framework            string `json:"framework"`
			Control              string `json:"control"`
			LeloirCapability     string `json:"leloir_capability"`
			CapabilityTestStatus string `json:"capability_test_status"`
			EvidenceCount        int    `json:"evidence_count"`
			TamperEvident        bool   `json:"tamper_evident"`
		} `json:"controls"`
	}
	body, err := c.getJSON("/compliance/evidence"+q, &e)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("tenant: %s\n", e.Tenant)
	printRow(pad("FRAMEWORK", 14), pad("CONTROL", 12), pad("CAPABILITY", 20), pad("TEST", 12), pad("EVID", 5), "TAMPER-EVIDENT")
	for _, ctl := range e.Controls {
		tamper := "no"
		if ctl.TamperEvident {
			tamper = "sí (hash-chain)"
		}
		printRow(pad(ctl.Framework, 14), pad(ctl.Control, 12), pad(trunc(ctl.LeloirCapability, 19), 20),
			pad(ctl.CapabilityTestStatus, 12), pad(fmt.Sprintf("%d", ctl.EvidenceCount), 5), tamper)
	}
	fmt.Printf("(%d controles)\n", len(e.Controls))
	return 0
}

func complianceBundle(c *client, q string) int {
	// The bundle IS the downloadable artifact — emit the full JSON to stdout (so
	// `leloir compliance bundle --framework EU-DORA > evidence.json` works), and a
	// one-line hash-chain verdict to stderr so the human sees it without parsing.
	body, status, err := c.do(http.MethodGet, "/compliance/bundle"+q, nil)
	if err != nil {
		return fail(err)
	}
	if status >= 400 {
		return fail(apiError(status, body))
	}
	var meta struct {
		HashChain struct {
			Enabled  bool   `json:"enabled"`
			Verified bool   `json:"verified"`
			Events   int    `json:"events"`
			Detail   string `json:"detail"`
		} `json:"hash_chain"`
	}
	_ = json.Unmarshal(body, &meta)
	verdict := "hash-chain M2 NO habilitada — evidencia no verificable criptográficamente"
	if meta.HashChain.Enabled {
		if meta.HashChain.Verified {
			verdict = fmt.Sprintf("hash-chain VERIFICADA ✓ (%d eventos, SHA-256 desde el génesis)", meta.HashChain.Events)
		} else {
			verdict = "hash-chain ROTA ✗ — " + meta.HashChain.Detail
		}
	}
	fmt.Fprintln(os.Stderr, "# "+verdict)
	if c.jsonOut {
		fmt.Println(strings.TrimSpace(string(body)))
	} else {
		printPrettyJSON(body)
	}
	return 0
}

// ─── leloir quarantine ────────────────────────────────────────────────────────

func cmdQuarantine(c *client) int {
	var q struct {
		Quarantined []struct {
			Tenant  string `json:"tenant"`
			Reason  string `json:"reason"`
			Signal  string `json:"signal"`
			Since   string `json:"since"`
			Blocked bool   `json:"blocked"`
		} `json:"quarantined"`
	}
	body, err := c.getJSON("/quarantine", &q)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	if len(q.Quarantined) == 0 {
		fmt.Println("sin tenants en cuarentena")
		return 0
	}
	for _, t := range q.Quarantined {
		printRow(pad(t.Tenant, 20), pad(t.Signal, 16), pad(trunc(t.Since, 19), 20), trunc(t.Reason, 50))
	}
	fmt.Printf("(%d en cuarentena)\n", len(q.Quarantined))
	return 0
}

// ─── leloir approvals ─────────────────────────────────────────────────────────
// El inbox HITL del tenant (gates pendientes). approve/reject viven en `inv`.

func cmdApprovals(c *client) int {
	var aps []struct {
		InvestigationID string `json:"InvestigationID"`
		Action          string `json:"Action"`
		BlastRadius     string `json:"BlastRadius"`
		Rationale       string `json:"Rationale"`
		Status          string `json:"Status"`
		CreatedAt       string `json:"CreatedAt"`
	}
	body, err := c.getJSON("/approvals", &aps)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	if len(aps) == 0 {
		fmt.Println("sin aprobaciones pendientes")
		return 0
	}
	for _, a := range aps {
		printRow(pad(trunc(a.InvestigationID, 24), 25), pad(a.Action, 22), pad(a.BlastRadius, 12), trunc(a.Rationale, 44))
	}
	fmt.Printf("(%d pendientes · aprobá con: leloir inv approve <id>)\n", len(aps))
	return 0
}
