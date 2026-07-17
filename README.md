# leloir CLI

The command-line client for [**Leloir**](https://github.com/villadalmine/leloir) — the
open-source **governance control plane for AI agents on Kubernetes**.

Everything Leloir does is a REST API (`/api/v1/*`). This CLI is a **pure client** of that
API — zero business logic, everything it does you could do with `curl`; the CLI just makes
it ergonomic. Trigger investigations and watch the governed agent reason **live**, inspect
audit and spend, manage API keys and contexts.

```console
$ leloir investigate "checkout pods crashlooping" --severity critical --follow
investigación inv-a1b2c3 → agente holmesgpt
◷ thought    the alert is on namespace 'shop'; I'll list the failing pods first
⚒ tool       get_pods {"namespace":"shop"}
✓ tool       ok (240ms)
$ llm.call   in=1210 out=180 $0.0011
⚒ tool       get_pod_logs {"namespace":"shop","name":"checkout-7c9f-2x"}
✓ tool       ok (180ms)
★ answer     [0.86]
  causa raíz:    OOMKilled — el límite de memoria (128Mi) es menor al working set (~210Mi)
  recomendación: subir resources.limits.memory a 256Mi en el Deployment checkout
■ complete   outcome=success tokens=1690 $0.0021
```

## Why a CLI (scope)

Leloir is **API-first**. The web UI, this CLI, CI pipelines and scripts are all clients of
the same governed API. The CLI's scope is deliberately narrow:

- **Operators / SRE** — trigger an investigation from a terminal and watch it live
  (`investigate --follow`), cancel/approve/reject, list what's running.
- **CISOs / compliance** — read the audit trail (`audit`) and per-tenant spend (`usage`).
- **Platform teams** — manage API keys (`apikeys`) and connection contexts (`config`),
  read the tenant's LLM identity (`llm-credentials`).
- **CI/CD** — everything with `--json` + meaningful exit codes.

**Non-goals:** no direct database or cluster access, no policy/CRD editing (that's
declarative via Kubernetes CRDs), no secrets ever printed. The CLI only ever talks to the
governed REST API — the same chokepoint every other client goes through.

## Install

```bash
# One-liner (detects OS/arch, verifies checksum)
curl -fsSL https://raw.githubusercontent.com/villadalmine/leloir-cli/main/install.sh | sh

# Go (needs Go 1.26+)
go install github.com/villadalmine/leloir-cli/cmd/leloir@latest

# Docker (distroless, multi-arch amd64+arm64)
docker run --rm ghcr.io/villadalmine/leloir-cli:latest version

# Or grab a prebuilt binary from the Releases page and put it on your PATH.
```

**Shell completions** (tab-complete commands + subcommands):

```bash
leloir completion bash > /etc/bash_completion.d/leloir      # bash
leloir completion zsh  > "${fpath[1]}/_leloir"              # zsh
leloir completion fish > ~/.config/fish/completions/leloir.fish
```

## Quickstart

```bash
# 1) Point the CLI at your control plane + your API key (kubeconfig-style contexts).
leloir config set-context prod \
  --server https://leloir.example.com \
  --api-key lk_xxxxxxxxxxxx
leloir config use-context prod

# 2) Fire an investigation and watch it live.
leloir investigate "high error rate on api-gateway" --severity critical --follow

# 3) Inspect.
leloir inv list
leloir usage
leloir audit --investigation inv-a1b2c3
```

No API key yet? In a `single-user` dev control plane you can point straight at it:

```bash
leloir --server http://localhost:8081 agents list
```

## Commands

| Command | What it does | API |
|---------|--------------|-----|
| `investigate "<title>" [--severity] [--description] [--label k=v] [--follow]` | Trigger an investigation; `--follow` streams events live | `POST /api/v1/alerts` (+ SSE) |
| `inv list [--limit --offset]` | Recent investigations (table) | `GET /investigations` |
| `inv get <id>` | One investigation in detail | `GET /investigations/{id}` |
| `inv stream <id>` | Attach to a running investigation's live stream | SSE |
| `inv cancel\|approve\|reject <id>` | Act on an investigation (approve = HITL gate) | `POST /investigations/{id}/…` |
| `agents list` / `agents get <name>` | Registered agents + health | `GET /agents` |
| `routes list` | Alert routes → agent + budget + match | `GET /routes` |
| `mcp-servers list` | Registered MCP servers (tools) | `GET /mcp-servers` |
| `apikeys create --name N [--role admin\|viewer] [--rate-per-minute N]` | Mint a key (shown **once**) | `POST /apikeys` |
| `apikeys list` / `apikeys revoke <id>` | List / revoke keys | `GET`/`DELETE /apikeys` |
| `usage` | Tenant metering vs budget (tokens/USD/investigations) | `GET /usage` |
| `llm-credentials` | The tenant's LLM endpoint + the Secret ref for its key (the key never travels through Leloir) | `GET /llm-credentials` |
| `audit [--investigation] [--type] [--limit]` | The audit trail (WORM) | `GET /audit` |
| `config view \| use-context <n> \| set-context <n> …` | Connection contexts (local file) | — |
| `version` | CLI version + server health | `GET /healthz` |

### Live investigation stream (`--follow`)

The differentiator: you see the **governed** agent work in real time — its reasoning,
every tool call through the MCP gateway, LLM cost per call, budget warnings, and any
human-in-the-loop approval gate:

```console
◷ thought    …            reasoning step
⚒ tool       name {args}  a tool call (routed + audited through the gateway)
✓ tool       ok (Nms)     tool succeeded
$ llm.call   in=… out=… $ per-call token + cost metering
! budget     resource N%  budget guard warning
? approval   PENDIENTE: … a HITL gate — approve with: leloir inv approve <id>
★ answer     [conf]       root cause + recommendation
■ complete   outcome=… …  final outcome, total tokens + cost
```

Exit code is **0** only if the investigation completed with `outcome=success` — so
`leloir investigate … --follow` is safe to use as a CI gate.

## Configuration

`~/.config/leloir/config.yaml` (mode `0600` — it can hold API keys, like a kubeconfig):

```yaml
current-context: prod
contexts:
  - name: prod
    server: https://leloir.example.com
    apiKey: lk_xxxxxxxxxxxx
  - name: dev            # single-user dev control plane (no key)
    server: http://localhost:8081
    tenant: acme         # only used in single-user mode (?tenant=)
```

Precedence: **flags** (`--server`/`--api-key`/`--context`) > **env**
(`LELOIR_SERVER`/`LELOIR_API_KEY`/`LELOIR_CONTEXT`/`LELOIR_CONFIG`) > **config file**.

## Scripting

`--json` emits the raw API response (pipe into `jq`); exit codes are meaningful:

```bash
leloir --json inv list | jq '.items[] | select(.Outcome=="cancelled") | .ID'
leloir --json usage    | jq '.usd_used'
```

| Exit | Meaning |
|------|---------|
| 0 | OK |
| 1 | API / network error |
| 2 | Usage error (bad flags) |
| 3 | `--follow` ended with an outcome other than `success` |

**Color** is on when writing to a terminal and off when piped or scripted — so `--json`
and `| jq` stay clean. Disable it with `NO_COLOR=1`; force it (e.g. into `less -R`) with
`LELOIR_FORCE_COLOR=1`.

## Architecture

```
you ──▶ leloir CLI ──HTTPS──▶ /api/v1/*  (the governed REST API)
           │                      │
     ~/.config/leloir       Leloir control plane
     (contexts, 0600)       (budget · HITL · audit WORM · MCP gateway)
```

The CLI holds no state beyond your local contexts. Auth is your API key
(`Authorization: Bearer lk_…`) or OIDC-fronted; the tenant is derived server-side from the
key. Nothing bypasses the control plane's guards.

## Development & releases

The CLI **source is maintained in the backend** (`leloir-core/cmd/leloir`, private), where
it lives next to the API it clients — an API change and its command ship together. This
repo **vendors** that source and adds the distribution (Dockerfile, CI, docs, versioning):

```bash
make sync           # pull the canonical source from leloir-core/cmd/leloir
make check-sync     # fail if the vendored copy drifted from the canonical source
make build test     # build (version injected) + test
make release-check  # gate before a release: test + no drift
```

Cut a release by bumping `VERSION` + `CHANGELOG.md`, then `git tag vX.Y.Z && git push --tags`
— the release workflow builds multi-arch binaries + a signed OCI image.

Apache 2.0 — see [LICENSE](LICENSE).
