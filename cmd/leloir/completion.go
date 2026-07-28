package main

import (
	"fmt"
	"os"
)

// Shell completion. The CLI is a hand-rolled dispatcher (no cobra), so the
// scripts are static — they know the command tree by hand. Install:
//
//	leloir completion bash > /etc/bash_completion.d/leloir      # bash
//	leloir completion zsh  > "${fpath[1]}/_leloir"              # zsh
//	leloir completion fish > ~/.config/fish/completions/leloir.fish

const compBash = `# bash completion for leloir
_leloir() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local commands="help version config investigate inv agents routes mcp-servers apikeys usage llm-credentials audit metrics roi compliance capabilities approvals quarantine license completion"
  local gflags="--context --server --api-key --json"
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=( $(compgen -W "$commands $gflags" -- "$cur") ); return
  fi
  case "${COMP_WORDS[1]}" in
    inv)         COMPREPLY=( $(compgen -W "list get stream cancel approve reject" -- "$cur") ) ;;
    agents)      COMPREPLY=( $(compgen -W "list get" -- "$cur") ) ;;
    apikeys)     COMPREPLY=( $(compgen -W "create list revoke" -- "$cur") ) ;;
    metrics)     COMPREPLY=( $(compgen -W "summary trends" -- "$cur") ) ;;
    compliance)  COMPREPLY=( $(compgen -W "evidence bundle" -- "$cur") ) ;;
    config)      COMPREPLY=( $(compgen -W "view use-context set-context" -- "$cur") ) ;;
    completion)  COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") ) ;;
    *)           COMPREPLY=( $(compgen -W "$gflags" -- "$cur") ) ;;
  esac
}
complete -F _leloir leloir
`

const compZsh = `#compdef leloir
_leloir() {
  local -a commands
  commands=(help version config investigate inv agents routes mcp-servers apikeys usage llm-credentials audit metrics roi compliance capabilities approvals quarantine license completion)
  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi
  case $words[2] in
    inv)        compadd list get stream cancel approve reject ;;
    agents)     compadd list get ;;
    apikeys)    compadd create list revoke ;;
    metrics)    compadd summary trends ;;
    compliance) compadd evidence bundle ;;
    config)     compadd view use-context set-context ;;
    completion) compadd bash zsh fish ;;
  esac
}
compdef _leloir leloir
`

const compFish = `# fish completion for leloir
complete -c leloir -f
complete -c leloir -n '__fish_use_subcommand' -a 'help version config investigate inv agents routes mcp-servers apikeys usage llm-credentials audit completion'
complete -c leloir -n '__fish_seen_subcommand_from inv'        -a 'list get stream cancel approve reject'
complete -c leloir -n '__fish_seen_subcommand_from agents'     -a 'list get'
complete -c leloir -n '__fish_seen_subcommand_from apikeys'    -a 'create list revoke'
complete -c leloir -n '__fish_seen_subcommand_from config'     -a 'view use-context set-context'
complete -c leloir -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`

func cmdCompletion(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "uso: leloir completion bash | zsh | fish")
		return 2
	}
	switch args[0] {
	case "bash":
		fmt.Print(compBash)
	case "zsh":
		fmt.Print(compZsh)
	case "fish":
		fmt.Print(compFish)
	default:
		fmt.Fprintf(os.Stderr, "error: shell desconocido %q (bash | zsh | fish)\n", args[0])
		return 2
	}
	return 0
}
