#!/usr/bin/env sh
# leloir CLI installer — detecta OS/arch, baja el binario del último release de
# GitHub, verifica el SHA256 y lo instala. Cliente puro de la API; cero runtime.
#
#   curl -fsSL https://raw.githubusercontent.com/villadalmine/leloir-cli/main/install.sh | sh
#
# Variables:
#   LELOIR_VERSION   tag a instalar (default: el último release)
#   LELOIR_BIN_DIR   destino (default: /usr/local/bin, o ~/.local/bin sin permiso)
set -eu

REPO="villadalmine/leloir-cli"
BIN="leloir"

err() { echo "leloir-install: $*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# ── OS / arch ────────────────────────────────────────────────────────────────
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux|darwin) ;;
  *) err "OS no soportado: $os (linux/darwin; en Windows usá el binario .exe del release)" ;;
esac
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) err "arch no soportada: $arch (amd64/arm64)" ;;
esac

have curl || have wget || err "necesito curl o wget"
get() { if have curl; then curl -fsSL "$1"; else wget -qO- "$1"; fi; }

# ── versión ──────────────────────────────────────────────────────────────────
ver="${LELOIR_VERSION:-}"
if [ -z "$ver" ]; then
  ver="$(get "https://api.github.com/repos/$REPO/releases/latest" \
        | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [ -n "$ver" ] || err "no pude resolver el último release (¿todavía no hay uno? seteá LELOIR_VERSION)"
fi
echo "leloir-install: $ver ($os/$arch)"

asset="${BIN}-${os}-${arch}"
base="https://github.com/$REPO/releases/download/$ver"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

get "$base/$asset"      > "$tmp/$BIN"      || err "no pude bajar $asset"
get "$base/SHA256SUMS"  > "$tmp/SHA256SUMS" 2>/dev/null || true

# ── verificar checksum (si el release lo publica) ────────────────────────────
if [ -s "$tmp/SHA256SUMS" ] && have sha256sum; then
  want="$(grep " $asset\$" "$tmp/SHA256SUMS" | awk '{print $1}')"
  got="$(sha256sum "$tmp/$BIN" | awk '{print $1}')"
  [ "$want" = "$got" ] || err "checksum NO coincide (esperado $want, got $got)"
  echo "leloir-install: checksum ✓"
fi

# ── instalar ─────────────────────────────────────────────────────────────────
chmod +x "$tmp/$BIN"
dir="${LELOIR_BIN_DIR:-/usr/local/bin}"
if [ ! -w "$dir" ] && [ -z "${LELOIR_BIN_DIR:-}" ]; then
  dir="$HOME/.local/bin"; mkdir -p "$dir"
fi
mv "$tmp/$BIN" "$dir/$BIN" || err "no pude escribir en $dir (probá con sudo o seteá LELOIR_BIN_DIR)"
echo "leloir-install: instalado en $dir/$BIN"
case ":$PATH:" in *":$dir:"*) ;; *) echo "leloir-install: agregá $dir a tu PATH" ;; esac
"$dir/$BIN" version 2>/dev/null | head -1 || true
