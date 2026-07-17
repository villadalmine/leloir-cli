package main

import (
	"strings"
	"testing"
)

func TestCompletion_Scripts(t *testing.T) {
	if !strings.Contains(compBash, "complete -F _leloir leloir") {
		t.Error("bash script sin la directiva complete")
	}
	if !strings.Contains(compZsh, "#compdef leloir") {
		t.Error("zsh script sin #compdef")
	}
	if !strings.Contains(compFish, "__fish_use_subcommand") {
		t.Error("fish script sin el predicado de subcomando")
	}
	// cada comando top-level debe aparecer en los 3 scripts
	scripts := map[string]string{"bash": compBash, "zsh": compZsh, "fish": compFish}
	for _, cmd := range []string{"investigate", "inv", "agents", "apikeys", "usage", "audit", "completion"} {
		for name, script := range scripts {
			if !strings.Contains(script, cmd) {
				t.Errorf("completion %s no incluye el comando %q", name, cmd)
			}
		}
	}
}

func TestCompletion_BadArgs(t *testing.T) {
	if got := cmdCompletion([]string{"bogus"}); got != 2 {
		t.Errorf("shell desconocido debe salir 2, got %d", got)
	}
	if got := cmdCompletion(nil); got != 2 {
		t.Errorf("sin arg debe salir 2, got %d", got)
	}
}
