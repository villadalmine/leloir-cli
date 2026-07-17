#!/usr/bin/env bash
# sync-from-core.sh — mantiene la fuente del CLI en step con su CANÓNICO.
#
# El código del CLI se MANTIENE en el backend privado (leloir-core/cmd/leloir), donde
# vive junto a la API que clientea (así un cambio de la API + su comando salen juntos).
# Este repo público VENDORIZA esa fuente + agrega la distribución (Dockerfile, CI, docs,
# versionado). Este script es el puente: evita el trap de "dos copias que divergen".
#
#   bash scripts/sync-from-core.sh          # copia la fuente del core → acá (al release)
#   bash scripts/sync-from-core.sh --check  # FALLA si la copia vendorizada quedó drift
#
# En CI del repo público el core no está montado → --check SKIP (no falla). El drift real
# se caza en local/pre-release, donde ambos repos son hermanos.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
CORE="${LELOIR_CORE:-$ROOT/../leloir/leloir-core}"
SRC="$CORE/cmd/leloir"
DST="$ROOT/cmd/leloir"
MODE="${1:-sync}"

if [ ! -d "$SRC" ]; then
  echo "⚠️  fuente canónica no encontrada ($SRC) — SKIP (¿repo privado leloir no montado?)."
  exit 0
fi

if [ "$MODE" = "--check" ]; then
  drift=0
  for f in "$SRC"/*.go; do
    b="$(basename "$f")"
    if ! cmp -s "$f" "$DST/$b"; then
      echo "  ✗ drift: cmd/leloir/$b difiere del canónico (leloir-core)"
      drift=1
    fi
  done
  # archivos que están acá pero ya no en el core (borrados upstream)
  for f in "$DST"/*.go; do
    b="$(basename "$f")"
    [ -f "$SRC/$b" ] || { echo "  ✗ sobrante: cmd/leloir/$b ya no existe en el canónico"; drift=1; }
  done
  if [ "$drift" = 0 ]; then
    echo "✅ el CLI vendorizado == el canónico (leloir-core/cmd/leloir)"; exit 0
  fi
  echo "   Corré: bash scripts/sync-from-core.sh  (y bumpeá la versión)"; exit 1
fi

# sync
cp "$SRC"/*.go "$DST/"
sha256sum "$SRC"/*.go | sed "s#$SRC/#cmd/leloir/#" > "$HERE/.source-hashes"
echo "✅ sincronizado $(ls "$SRC"/*.go | wc -l) archivos de leloir-core/cmd/leloir → cmd/leloir/"
echo "   revisá git diff + bumpeá VERSION + CHANGELOG antes de release."
