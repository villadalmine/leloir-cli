package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

// ─── leloir agents ────────────────────────────────────────────────────────────

func cmdAgents(c *client, args []string) int {
	if len(args) == 0 || args[0] == "list" {
		var agents []struct {
			Name     string `json:"Name"`
			TenantID string `json:"TenantID"`
			Version  string `json:"Version"`
			Health   string `json:"Health"`
		}
		body, err := c.getJSON("/agents", &agents)
		if err != nil {
			return fail(err)
		}
		if c.emitJSON(body) {
			return 0
		}
		header(pad("NAME", 20), pad("TENANT", 14), pad("VERSION", 10), "HEALTH")
		for _, a := range agents {
			printRow(pad(a.Name, 20), pad(a.TenantID, 14), pad(a.Version, 10), col(statusCode(a.Health), a.Health))
		}
		return 0
	}
	if args[0] == "get" && len(args) == 2 {
		body, err := c.getJSON("/agents/"+url.PathEscape(args[1]), nil)
		if err != nil {
			return fail(err)
		}
		printPrettyJSON(body)
		return 0
	}
	fmt.Fprintln(os.Stderr, "uso: leloir agents list | get <name>")
	return 2
}

// ─── leloir routes ────────────────────────────────────────────────────────────

func cmdRoutes(c *client, args []string) int {
	var routes []struct {
		Name            string            `json:"Name"`
		TenantID        string            `json:"TenantID"`
		Enabled         bool              `json:"Enabled"`
		Priority        int32             `json:"Priority"`
		MatchLabels     map[string]string `json:"MatchLabels"`
		AgentName       string            `json:"AgentName"`
		BudgetMaxUSD    float64           `json:"BudgetMaxUSD"`
		BudgetMaxTokens int64             `json:"BudgetMaxTokens"`
		AgentHealth     string            `json:"agentHealth"`
	}
	body, err := c.getJSON("/routes", &routes)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	header(pad("NAME", 20), pad("TENANT", 14), pad("AGENT", 16), pad("HEALTH", 10), pad("BUDGET", 16), "MATCH")
	for _, r := range routes {
		match := "todos"
		if len(r.MatchLabels) > 0 {
			m, _ := json.Marshal(r.MatchLabels)
			match = trunc(string(m), 40)
		}
		budget := fmt.Sprintf("$%.2f/%dk tok", r.BudgetMaxUSD, r.BudgetMaxTokens/1000)
		printRow(pad(r.Name, 20), pad(r.TenantID, 14), pad(r.AgentName, 16),
			col(statusCode(r.AgentHealth), pad(r.AgentHealth, 10)), pad(budget, 16), match)
	}
	return 0
}

// ─── leloir mcp-servers ───────────────────────────────────────────────────────

func cmdMCPServers(c *client, args []string) int {
	var servers []map[string]any
	body, err := c.getJSON("/mcp-servers", &servers)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	if len(servers) == 0 {
		fmt.Println("(sin MCP servers registrados — se poblarán desde MCPServer CRDs)")
		return 0
	}
	printPrettyJSON(body)
	return 0
}

// ─── leloir apikeys ───────────────────────────────────────────────────────────

func cmdAPIKeys(c *client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: leloir apikeys create --name N [--role admin|viewer] [--rate-per-minute N] | list | revoke <id>")
		return 2
	}
	switch args[0] {
	case "create":
		fs := newFlagSet("apikeys create")
		name := fs.String("name", "", "etiqueta humana (ej: ci-bot)")
		role := fs.String("role", "viewer", "admin | viewer")
		rpm := fs.Int("rate-per-minute", 0, "rate limit propio (0 = default del server)")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if *name == "" {
			fmt.Fprintln(os.Stderr, "error: --name es obligatorio")
			return 2
		}
		body, status, err := c.do(http.MethodPost, "/apikeys",
			map[string]any{"name": *name, "role": *role, "ratePerMinute": *rpm})
		if err != nil {
			return fail(err)
		}
		if status >= 400 {
			return fail(apiError(status, body))
		}
		if c.emitJSON(body) {
			return 0
		}
		var k struct {
			ID   string `json:"id"`
			Role string `json:"role"`
			Key  string `json:"key"`
		}
		_ = json.Unmarshal(body, &k)
		fmt.Printf("id:   %s\nrole: %s\nkey:  %s\n\n", k.ID, k.Role, col(cAnswer, k.Key))
		fmt.Println(col(cWarn, "⚠ esta key se muestra UNA sola vez — guardala ya:"))
		fmt.Printf("  leloir config set-context <nombre> --server %s --api-key %s\n", c.server, k.Key)
		return 0

	case "list":
		var page struct {
			Items []struct {
				ID         string  `json:"id"`
				Name       string  `json:"name"`
				Role       string  `json:"role"`
				RatePerMin int     `json:"rate_per_minute"`
				CreatedAt  string  `json:"created_at"`
				LastUsedAt *string `json:"last_used_at"`
				RevokedAt  *string `json:"revoked_at"`
			} `json:"items"`
		}
		body, err := c.getJSON("/apikeys", &page)
		if err != nil {
			return fail(err)
		}
		if c.emitJSON(body) {
			return 0
		}
		header(pad("ID", 18), pad("NAME", 16), pad("ROLE", 8), pad("RPM", 6), pad("STATE", 8), "LAST USED")
		for _, k := range page.Items {
			state := "active"
			if k.RevokedAt != nil {
				state = "revoked"
			}
			last := "-"
			if k.LastUsedAt != nil {
				last = trunc(*k.LastUsedAt, 19)
			}
			rpm := "def"
			if k.RatePerMin > 0 {
				rpm = fmt.Sprintf("%d", k.RatePerMin)
			}
			printRow(pad(k.ID, 18), pad(k.Name, 16), pad(k.Role, 8), pad(rpm, 6),
				col(statusCode(state), pad(state, 8)), last)
		}
		return 0

	case "revoke":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "uso: leloir apikeys revoke <id>")
			return 2
		}
		body, status, err := c.do(http.MethodDelete, "/apikeys/"+url.PathEscape(args[1]), nil)
		if err != nil {
			return fail(err)
		}
		if status >= 400 {
			return fail(apiError(status, body))
		}
		fmt.Println("revocada — cualquier request con esa key recibe 401 desde ahora")
		return 0

	default:
		fmt.Fprintf(os.Stderr, "error: subcomando desconocido %q\n", args[0])
		return 2
	}
}

// ─── leloir usage ─────────────────────────────────────────────────────────────

func cmdUsage(c *client, args []string) int {
	var u struct {
		TenantID           string  `json:"tenant_id"`
		PeriodStart        *string `json:"period_start"`
		TokensUsed         int64   `json:"tokens_used"`
		USDUsed            float64 `json:"usd_used"`
		InvestigationCount int64   `json:"investigation_count"`
		Budget             struct {
			MonthlyMaxTokens int64   `json:"monthly_max_tokens"`
			MonthlyMaxUSD    float64 `json:"monthly_max_usd"`
			HardLimitAction  string  `json:"hard_limit_action"`
		} `json:"budget"`
	}
	body, err := c.getJSON("/usage", &u)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("tenant: %s\n", u.TenantID)
	if u.PeriodStart != nil {
		fmt.Printf("período desde: %s\n", trunc(*u.PeriodStart, 10))
	}
	fmt.Printf("investigaciones: %d\n", u.InvestigationCount)
	fmt.Printf("tokens: %s\n", quota(u.TokensUsed, u.Budget.MonthlyMaxTokens))
	fmt.Printf("USD:    %s\n", quotaF(u.USDUsed, u.Budget.MonthlyMaxUSD))
	if u.Budget.HardLimitAction != "" {
		fmt.Printf("al límite: %s\n", u.Budget.HardLimitAction)
	}
	return 0
}

func quota(used, max int64) string {
	if max <= 0 {
		return fmt.Sprintf("%d (sin límite)", used)
	}
	return col(pctColor(float64(used), float64(max)),
		fmt.Sprintf("%d / %d (%.1f%%)", used, max, float64(used)/float64(max)*100))
}

func quotaF(used, max float64) string {
	if max <= 0 {
		return fmt.Sprintf("$%.4f (sin límite)", used)
	}
	return col(pctColor(used, max),
		fmt.Sprintf("$%.4f / $%.2f (%.1f%%)", used, max, used/max*100))
}

// ─── leloir audit ─────────────────────────────────────────────────────────────

func cmdAudit(c *client, args []string) int {
	fs := newFlagSet("audit")
	invID := fs.String("investigation", "", "filtrar por investigación")
	evType := fs.String("type", "", "filtrar por tipo de evento")
	limit := fs.Int("limit", 50, "máximo de eventos")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", *limit))
	if *invID != "" {
		q.Set("investigation_id", *invID)
	}
	if *evType != "" {
		q.Set("event_type", *evType)
	}
	var events []struct {
		Type            string         `json:"Type"`
		Timestamp       string         `json:"Timestamp"`
		UserSubject     string         `json:"UserSubject"`
		InvestigationID string         `json:"InvestigationID"`
		Details         map[string]any `json:"Details"`
	}
	body, err := c.getJSON("/audit?"+q.Encode(), &events)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	for _, e := range events {
		det := ""
		if len(e.Details) > 0 {
			d, _ := json.Marshal(e.Details)
			det = trunc(string(d), 70)
		}
		printRow(pad(trunc(e.Timestamp, 19), 20), pad(e.Type, 28), pad(trunc(e.InvestigationID, 24), 25), pad(trunc(e.UserSubject, 18), 19), det)
	}
	fmt.Printf("(%d eventos)\n", len(events))
	return 0
}

// printPrettyJSON re-indents a JSON body for humans.
func printPrettyJSON(body []byte) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		fmt.Println(string(body))
		return
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(out))
}

// ─── leloir llm-credentials ───────────────────────────────────────────────────

// cmdLLMCredentials prints the tenant's LLM identity (spec-m15 15.4): the
// endpoint to point an agent at + the REFERENCE to the Secret with its virtual
// key. The key value never travels through Leloir — the hint shows how the
// tenant-owner reads it with their own kubectl RBAC.
func cmdLLMCredentials(c *client) int {
	var lc struct {
		TenantID           string `json:"tenant_id"`
		Endpoint           string `json:"endpoint"`
		KeySecretName      string `json:"key_secret_name"`
		KeySecretNamespace string `json:"key_secret_namespace"`
		Driver             string `json:"driver"`
	}
	body, err := c.getJSON("/llm-credentials", &lc)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	fmt.Printf("tenant:   %s\n", lc.TenantID)
	fmt.Printf("endpoint: %s\n", lc.Endpoint)
	fmt.Printf("driver:   %s\n", lc.Driver)
	if lc.KeySecretName != "" {
		fmt.Printf("key:      Secret %s/%s (key \"api_key\") — nunca viaja por Leloir\n",
			lc.KeySecretNamespace, lc.KeySecretName)
		fmt.Printf("\nPara configurar tu agente (con TU RBAC):\n")
		fmt.Printf("  OPENAI_API_BASE=%s\n", lc.Endpoint)
		fmt.Printf("  OPENAI_API_KEY=$(kubectl get secret %s -n %s -o jsonpath='{.data.api_key}' | base64 -d)\n",
			lc.KeySecretName, lc.KeySecretNamespace)
	}
	return 0
}
