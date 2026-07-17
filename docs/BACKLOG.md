# Backlog — Leloir CLI

## Hecho (v0.1.0, 2026-07-16)
- [x] **Extracción:** el código de la CLI (leloir-core/cmd/leloir) vendorizado acá con
      go.mod propio; canónico sigue en el core (self-managed por `scripts/sync-from-core.sh`
      + `make check-sync` para evitar drift). Buildea + testea.
- [x] **Configuración:** contexts kubeconfig-style (~/.config/leloir/config.yaml, 0600),
      auth por API key, precedencia flags>env>file.
- [x] **Comandos base:** investigate (--follow live stream), inv, agents, routes, mcp-servers,
      apikeys, usage, llm-credentials, audit, config, version.
- [x] **Distribución:** Dockerfile multi-arch (cross-compile), CI (build/test/drift), release
      (binarios multi-plataforma + imagen OCI firmada cosign), README completo.

## Pendiente
- [ ] **Publicar el repo** en GitHub (público) + primer release `v0.1.0` (tag → workflow).
- [ ] Shell completions (bash/zsh/fish) + `leloir completion`.
- [ ] Homebrew tap / Scoop manifest para install de un comando.
- [ ] `--watch` para `inv list` (refresco en vivo del tablero de investigaciones).
- [ ] Cuando el backend agregue endpoints, el comando sale del core y se sincroniza acá.
