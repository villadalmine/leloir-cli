package main

import (
	"strings"
	"testing"
)

func TestColor_RespectsEnabled(t *testing.T) {
	defer func(v bool) { colorEnabled = v }(colorEnabled)

	colorEnabled = false
	if got := col(cOk, "ok"); got != "ok" {
		t.Errorf("color OFF debe devolver el texto crudo, got %q", got)
	}

	colorEnabled = true
	got := col(cOk, "ok")
	if !strings.HasPrefix(got, "\033[") || !strings.HasSuffix(got, cReset) {
		t.Errorf("color ON debe envolver en ANSI, got %q", got)
	}
	// código vacío = sin envolver (evita un reset suelto)
	if got := col("", "x"); got != "x" {
		t.Errorf("código vacío no debe agregar ANSI, got %q", got)
	}
}

func TestColor_StatusCode(t *testing.T) {
	cases := map[string]string{
		"success": cOk, "healthy": cOk, "active": cOk,
		"running": cWarn, "degraded": cWarn,
		"failed": cErr, "cancelled": cErr, "revoked": cErr,
		"weird": "", "": "",
	}
	for status, want := range cases {
		if got := statusCode(status); got != want {
			t.Errorf("statusCode(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestColor_PctThresholds(t *testing.T) {
	if pctColor(10, 0) != cDim { // sin límite
		t.Error("sin límite (max<=0) debe ser dim")
	}
	if pctColor(50, 100) != cOk { // 50% ok
		t.Error("50% debe ser verde")
	}
	if pctColor(85, 100) != cWarn { // 85% warning
		t.Error("85% debe ser ámbar")
	}
	if pctColor(120, 100) != cErr { // 120% over
		t.Error(">100% debe ser rojo")
	}
}
