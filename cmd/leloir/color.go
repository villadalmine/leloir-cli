package main

// ANSI color for the human-facing output. The palette matches the CLI's event
// vocabulary (◷ thought · ⚒ tool · ✓ ok · $ llm.call · ! budget · ? approval ·
// ★ answer · ■ complete) so a live investigation reads at a glance.
//
// Color is ON only when stdout is a real terminal, NO_COLOR is unset, and --json
// is off — piped/scripted output stays plain (the NO_COLOR convention). Pure
// stdlib: TTY is detected via the char-device bit, no extra dependency.

import "os"

var colorEnabled = detectColor()

func detectColor() bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	if os.Getenv("LELOIR_FORCE_COLOR") == "1" { // tests + `| less -R`
		return true
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// 256-color codes, tuned to the same palette as the web mockup.
const (
	cReset   = "\033[0m"
	cDim     = "\033[38;5;244m" // muted grey — secondary text
	cThought = "\033[38;5;74m"  // steel blue — reasoning
	cTool    = "\033[38;5;179m" // amber — a tool call
	cOk      = "\033[38;5;72m"  // green — success
	cErr     = "\033[38;5;167m" // soft red — failure
	cCost    = "\033[38;5;140m" // violet — llm cost
	cWarn    = "\033[38;5;179m" // amber — budget warning
	cAppr    = "\033[38;5;175m" // pink — approval gate
	cAnswer  = "\033[1;38;5;179m" // bold amber — the answer / star
	cBold    = "\033[1m"
)

// col wraps s in an ANSI code when color is enabled, otherwise returns s as-is.
func col(code, s string) string {
	if !colorEnabled || code == "" {
		return s
	}
	return code + s + cReset
}

// statusCode returns the ANSI code for a status/outcome word (green ok · amber
// active · red bad), or "" for default. Use as col(statusCode(x), pad(x, w)) so
// the PADDING happens before the codes and columns stay aligned.
func statusCode(s string) string {
	switch s {
	case "success", "completed", "healthy", "active", "ok":
		return cOk
	case "running", "pending", "degraded":
		return cWarn
	case "cancelled", "failed", "error", "unhealthy", "off-radar", "revoked":
		return cErr
	}
	return ""
}

// header dims a table header row (the padded column labels).
func header(cols ...string) {
	printRow(col(cDim, joinCols(cols)))
}

func joinCols(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += "  "
		}
		out += c
	}
	return out
}

// pctColor tints a "used %" number: green under 80, amber under 100, red at/over.
func pctColor(used, max float64) string {
	code := cOk
	switch {
	case max <= 0:
		return cDim
	case used/max >= 1:
		code = cErr
	case used/max >= 0.8:
		code = cWarn
	}
	return code
}
