# Changelog — leloir CLI

Versionado semántico. Cada release es un tag `vX.Y.Z` que dispara el workflow de
release (binarios multi-plataforma + imagen OCI firmada). La fuente del CLI se
mantiene en el backend (leloir-core/cmd/leloir) y se vendoriza acá con
`make sync`; `make release-check` garantiza que no haya drift antes de tagear.

## v0.1.0 — 2026-07-16

Primer corte público del CLI, extraído del monorepo (leloir-core/cmd/leloir).

- Cliente HTTP puro de la REST API del control plane (`/api/v1/*`) — cero lógica
  de negocio: todo lo que hace se puede hacer con `curl`, el CLI lo hace ergonómico.
- Comandos: `investigate` (con `--follow` = stream vivo del agente gobernado),
  `inv` (list/get/stream/cancel/approve/reject), `agents`, `routes`, `mcp-servers`,
  `apikeys` (create/list/revoke), `usage`, `llm-credentials`, `audit`, `config`
  (contexts kubeconfig-style), `version`.
- Config kubeconfig-style en `~/.config/leloir/config.yaml` (0600), multi-context.
  Precedencia flags > env (`LELOIR_SERVER`/`LELOIR_API_KEY`/`LELOIR_CONTEXT`) > file.
- `--json` para scripting; exit codes CI-friendly (0 ok · 1 API/red · 2 uso ·
  3 `--follow` terminó sin success).
- Distribución: binario estático (`go install` / release), imagen distroless
  multi-arch (amd64+arm64, cross-compilada sin emulación), firma cosign keyless.
