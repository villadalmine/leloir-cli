// Command leloir is the Leloir CLI — a pure client of the control plane
// REST API (docs/specs/spec-m9-cli.md). Zero business logic lives here:
// everything the CLI does can be done with curl; the CLI makes it ergonomic.
//
// Usage:
//
//	leloir investigate "pod crashlooping" --severity critical --follow
//	leloir inv list
//	leloir --context team-demo usage
//
// Exit codes: 0 ok · 1 API/network error · 2 usage error · 3 --follow ended
// with an outcome other than success.
package main

import (
	"fmt"
	"os"
	"strings"
)

var version = "dev"

const rootUsage = `leloir — CLI del control plane Leloir (cliente puro de la REST API)

Comandos:
  investigate <título>   dispara una investigación (--follow para verla en vivo)
  inv                    list | get | stream | cancel | approve | reject
  agents                 list | get <name>
  routes                 list
  mcp-servers            list
  apikeys                create | list | revoke <id>
  usage                  metering del tenant (tokens/USD/investigaciones vs budget)
  llm-credentials        identidad LLM del tenant: endpoint + ref al Secret de la key
  audit                  eventos de auditoría (--investigation, --type, --limit)
  config                 view | use-context <name> | set-context <name> ...
  completion             script de completado para bash | zsh | fish
  version                versión del CLI + salud del server

Flags globales (antes o después del comando):
  --context NAME   context del archivo ~/.config/leloir/config.yaml
  --server URL     override del server (env: LELOIR_SERVER)
  --api-key KEY    override de la API key lk_... (env: LELOIR_API_KEY)
  --json           salida cruda de la API (scripteable)
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	g, rest, err := extractGlobals(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if len(rest) == 0 {
		fmt.Print(rootUsage)
		return 2
	}

	c, err := newClient(g)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	cmd, sub := rest[0], rest[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(rootUsage)
		return 0
	case "version":
		return cmdVersion(c)
	case "config":
		return cmdConfig(g, sub)
	case "investigate":
		return cmdInvestigate(c, sub)
	case "inv", "investigations":
		return cmdInv(c, sub)
	case "agents":
		return cmdAgents(c, sub)
	case "routes":
		return cmdRoutes(c, sub)
	case "mcp-servers":
		return cmdMCPServers(c, sub)
	case "apikeys":
		return cmdAPIKeys(c, sub)
	case "usage":
		return cmdUsage(c, sub)
	case "llm-credentials", "llm-creds":
		return cmdLLMCredentials(c)
	case "audit":
		return cmdAudit(c, sub)
	case "completion":
		return cmdCompletion(sub)
	default:
		fmt.Fprintf(os.Stderr, "error: comando desconocido %q\n\n%s", cmd, rootUsage)
		return 2
	}
}

// globals are the flags accepted anywhere on the command line.
type globals struct {
	context string
	server  string
	apiKey  string
	jsonOut bool
}

// extractGlobals pulls global flags out of args (they may appear before or
// after the subcommand) and returns the remaining args in order.
func extractGlobals(args []string) (globals, []string, error) {
	var g globals
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, val, hasVal := strings.Cut(a, "=")
		takeValue := func() (string, error) {
			if hasVal {
				return val, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag %s requiere un valor", name)
			}
			i++
			return args[i], nil
		}
		switch name {
		case "--context":
			v, err := takeValue()
			if err != nil {
				return g, nil, err
			}
			g.context = v
		case "--server":
			v, err := takeValue()
			if err != nil {
				return g, nil, err
			}
			g.server = v
		case "--api-key":
			v, err := takeValue()
			if err != nil {
				return g, nil, err
			}
			g.apiKey = v
		case "--json":
			g.jsonOut = true
		default:
			rest = append(rest, a)
			// "config" manages contexts itself: --server/--api-key after it
			// belong to set-context, not to the global connection flags.
			if len(rest) == 1 && a == "config" {
				rest = append(rest, args[i+1:]...)
				return g, rest, nil
			}
		}
	}
	return g, rest, nil
}

func cmdVersion(c *client) int {
	fmt.Printf("leloir CLI %s\n", version)
	fmt.Printf("server: %s\n", c.server)
	body, err := c.rawGet("/healthz")
	if err != nil {
		fmt.Printf("health: unreachable (%v)\n", err)
		return 1
	}
	fmt.Printf("health: %s\n", strings.TrimSpace(string(body)))
	return 0
}
