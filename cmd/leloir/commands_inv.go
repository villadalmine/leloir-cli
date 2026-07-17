package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ─── leloir investigate ───────────────────────────────────────────────────────

type labelFlags map[string]string

func (l labelFlags) String() string { return "" }
func (l labelFlags) Set(v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok || k == "" {
		return fmt.Errorf("label debe ser k=v, recibí %q", v)
	}
	l[k] = val
	return nil
}

func cmdInvestigate(c *client, args []string) int {
	fs := newFlagSet("investigate")
	description := fs.String("description", "", "descripción / contexto de la alerta")
	severity := fs.String("severity", "warning", "critical | warning | info")
	source := fs.String("source", "cli", "origen de la alerta")
	follow := fs.Bool("follow", false, "stream de eventos hasta complete (exit 0 solo si success)")
	labels := labelFlags{}
	fs.Var(labels, "label", "label k=v (repetible)")

	// Go's flag package stops at the first positional arg, so accept the
	// title either first ("<título>" --follow) or after the flags.
	title := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		title = args[0]
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if title == "" && fs.NArg() == 1 {
		title = fs.Arg(0)
	}
	if title == "" || fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, `uso: leloir investigate "<título>" [--description ...] [--severity ...] [--label k=v] [--follow]`)
		return 2
	}

	payload := map[string]any{
		"source":   *source,
		"title":    title,
		"severity": *severity,
	}
	if *description != "" {
		payload["description"] = *description
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}

	body, status, err := c.do(http.MethodPost, "/alerts", payload)
	if err != nil {
		return fail(err)
	}
	if status >= 400 {
		return fail(apiError(status, body))
	}
	var resp struct {
		InvestigationID string `json:"investigation_id"`
		Agent           string `json:"agent"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fail(fmt.Errorf("respuesta inesperada: %w", err))
	}
	if c.emitJSON(body) && !*follow {
		return 0
	}
	if !c.jsonOut {
		fmt.Printf("investigación %s → agente %s\n", resp.InvestigationID, resp.Agent)
	}
	if !*follow {
		if !c.jsonOut {
			fmt.Printf("seguíla con: leloir inv stream %s\n", resp.InvestigationID)
		}
		return 0
	}
	return c.streamInvestigation(resp.InvestigationID)
}

// ─── leloir inv ───────────────────────────────────────────────────────────────

func cmdInv(c *client, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: leloir inv list | get <id> | stream <id> | cancel <id> | approve <id> | reject <id>")
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdInvList(c, rest)
	case "get":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "uso: leloir inv get <id>")
			return 2
		}
		body, err := c.getJSON("/investigations/"+rest[0], nil)
		if err != nil {
			return fail(err)
		}
		if c.emitJSON(body) {
			return 0
		}
		var inv invRecord
		_ = json.Unmarshal(body, &inv)
		printInvDetail(inv)
		return 0
	case "stream":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "uso: leloir inv stream <id>")
			return 2
		}
		return c.streamInvestigation(rest[0])
	case "cancel", "approve", "reject":
		if len(rest) != 1 {
			fmt.Fprintf(os.Stderr, "uso: leloir inv %s <id>\n", sub)
			return 2
		}
		body, status, err := c.do(http.MethodPost, "/investigations/"+rest[0]+"/"+sub, map[string]any{})
		if err != nil {
			return fail(err)
		}
		if status >= 400 {
			return fail(apiError(status, body))
		}
		fmt.Printf("%s: ok\n", sub)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: subcomando desconocido %q\n", sub)
		return 2
	}
}

type invRecord struct {
	ID          string  `json:"ID"`
	ParentID    string  `json:"ParentID"`
	TenantID    string  `json:"TenantID"`
	AgentName   string  `json:"AgentName"`
	Status      string  `json:"Status"`
	Outcome     string  `json:"Outcome"`
	Reason      string  `json:"Reason"`
	TotalUSD    float64 `json:"TotalUSD"`
	TotalTokens int64   `json:"TotalTokens"`
	Started     string  `json:"Started"`
	Completed   *string `json:"Completed"`
}

func cmdInvList(c *client, args []string) int {
	fs := newFlagSet("inv list")
	limit := fs.Int("limit", 20, "máximo de resultados")
	offset := fs.Int("offset", 0, "offset de paginación")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := fmt.Sprintf("/investigations?limit=%d&offset=%d", *limit, *offset)
	var page struct {
		Items   []invRecord `json:"items"`
		Total   int64       `json:"total"`
		HasMore bool        `json:"has_more"`
	}
	body, err := c.getJSON(path, &page)
	if err != nil {
		return fail(err)
	}
	if c.emitJSON(body) {
		return 0
	}
	printRow(pad("ID", 26), pad("STATUS", 10), pad("OUTCOME", 12), pad("AGENT", 14), pad("TOKENS", 8), "STARTED")
	for _, inv := range page.Items {
		printRow(pad(inv.ID, 26), pad(inv.Status, 10), pad(inv.Outcome, 12),
			pad(inv.AgentName, 14), pad(fmt.Sprintf("%d", inv.TotalTokens), 8), trunc(inv.Started, 19))
	}
	fmt.Printf("(%d de %d)%s\n", len(page.Items), page.Total, map[bool]string{true: " — hay más: --offset", false: ""}[page.HasMore])
	return 0
}

func printInvDetail(inv invRecord) {
	fmt.Printf("ID:       %s\n", inv.ID)
	if inv.ParentID != "" {
		fmt.Printf("Parent:   %s (sub-investigación A2A)\n", inv.ParentID)
	}
	fmt.Printf("Tenant:   %s\nAgente:   %s\nStatus:   %s\n", inv.TenantID, inv.AgentName, inv.Status)
	if inv.Outcome != "" {
		fmt.Printf("Outcome:  %s\n", inv.Outcome)
	}
	if inv.Reason != "" {
		fmt.Printf("Reason:   %s\n", inv.Reason)
	}
	fmt.Printf("Tokens:   %d\nUSD:      %.4f\nStarted:  %s\n", inv.TotalTokens, inv.TotalUSD, inv.Started)
	if inv.Completed != nil {
		fmt.Printf("Completed:%s\n", *inv.Completed)
	}
}

// ─── SSE streaming ────────────────────────────────────────────────────────────

// streamInvestigation renders the live event stream. Returns 0 when the
// investigation completes with outcome success, 3 for any other outcome,
// 1 on transport errors — the CI contract from spec-m9-cli.md.
func (c *client) streamInvestigation(id string) int {
	req, err := http.NewRequest(http.MethodGet, c.apiURL("/investigations/"+id+"/stream"), nil)
	if err != nil {
		return fail(err)
	}
	c.setAuth(req)
	// No timeout: streams last as long as the investigation.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fail(fmt.Errorf("stream: API %d", resp.StatusCode))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var evt struct {
			Type    string          `json:"Type"`
			Payload json.RawMessage `json:"Payload"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &evt); err != nil {
			continue
		}
		if c.jsonOut {
			fmt.Println(strings.TrimSpace(line[5:]))
		}
		done, code := renderEvent(id, evt.Type, evt.Payload, c.jsonOut)
		if done {
			return code
		}
	}
	if err := scanner.Err(); err != nil {
		return fail(fmt.Errorf("stream interrumpido: %w", err))
	}
	// Stream closed without a complete event: investigation may have finished
	// earlier (broker replays only live events) — report the stored state.
	var inv invRecord
	if _, err := c.getJSON("/investigations/"+id, &inv); err == nil && inv.Status != "running" {
		if !c.jsonOut {
			fmt.Printf("■ %s outcome=%s tokens=%d\n", inv.Status, inv.Outcome, inv.TotalTokens)
		}
		if inv.Outcome == "success" {
			return 0
		}
		return 3
	}
	return 0
}

// renderEvent prints one line per event. Returns (true, exitCode) on complete.
func renderEvent(invID, typ string, raw json.RawMessage, jsonOut bool) (bool, int) {
	get := func(v any) { _ = json.Unmarshal(raw, v) }
	line := ""
	switch typ {
	case "thought":
		var p struct{ Content string }
		get(&p)
		line = "◷ thought    " + trunc(p.Content, 100)
	case "progress":
		var p struct{ Message string }
		get(&p)
		line = "◷ progress   " + trunc(p.Message, 100)
	case "tool.request":
		var p struct {
			ToolName string
			Args     map[string]any
		}
		get(&p)
		args, _ := json.Marshal(p.Args)
		line = "⚒ tool       " + p.ToolName + " " + trunc(string(args), 80)
	case "tool.response":
		var p struct {
			Success    bool
			Error      string
			DurationMS int64
		}
		get(&p)
		if p.Success {
			line = fmt.Sprintf("✓ tool       ok (%dms)", p.DurationMS)
		} else {
			line = "✗ tool       " + trunc(p.Error, 90)
		}
	case "llm.call":
		var p struct {
			InputTokens, OutputTokens int
			CostUSD                   float64
		}
		get(&p)
		line = fmt.Sprintf("$ llm.call   in=%d out=%d $%.4f", p.InputTokens, p.OutputTokens, p.CostUSD)
	case "budget.warning":
		var p struct {
			Resource    string
			UsedPercent float64
		}
		get(&p)
		line = fmt.Sprintf("! budget     %s %.0f%% usado", p.Resource, p.UsedPercent*100)
	case "approval.request":
		var p struct{ Action, Rationale string }
		get(&p)
		line = fmt.Sprintf("? approval   PENDIENTE: %s — aprobá con: leloir inv approve %s", p.Action, invID)
	case "error":
		var p struct {
			Code, Message string
			Recoverable   bool
		}
		get(&p)
		line = fmt.Sprintf("✗ error      [%s] %s", p.Code, trunc(p.Message, 90))
	case "answer":
		var p struct {
			Summary, RootCause, Recommendation string
			Confidence                         float64
		}
		get(&p)
		if !jsonOut {
			fmt.Printf("★ answer     [%.2f]\n", p.Confidence)
			printBlock("  causa raíz:    ", p.RootCause)
			printBlock("  recomendación: ", p.Recommendation)
		}
		return false, 0
	case "complete":
		var p struct {
			Outcome     string
			TotalTokens int64
			TotalCost   float64
			Reason      string
		}
		get(&p)
		if !jsonOut {
			line = fmt.Sprintf("■ complete   outcome=%s tokens=%d $%.4f", p.Outcome, p.TotalTokens, p.TotalCost)
			if p.Reason != "" {
				line += " (" + p.Reason + ")"
			}
			fmt.Println(line)
		}
		if p.Outcome == "success" {
			return true, 0
		}
		return true, 3
	default:
		line = "· " + typ
	}
	if line != "" && !jsonOut {
		fmt.Println(line)
	}
	return false, 0
}

func printBlock(prefix, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	lines := strings.Split(text, "\n")
	fmt.Println(prefix + lines[0])
	indent := strings.Repeat(" ", len([]rune(prefix)))
	for _, l := range lines[1:] {
		fmt.Println(indent + l)
	}
}
