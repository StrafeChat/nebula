#!/usr/bin/env bash
#
# Fetch the Unicode emoji artwork sets nebula seeds into its object store, so an instance
# serves emoji from its own CDN (/v1/emoji/<set>/…) instead of a third-party host. Run at
# image-build time (see Dockerfile); the result is a directory nebula's emoji seeder copies
# into storage on first start.
#
#   ./scripts/fetch-emoji.sh [OUT_DIR]      # OUT_DIR defaults to ./emoji
#
# Each set is pinned to an exact upstream version. The file-name layout is preserved exactly
# as upstream ships it, because the web client addresses each set by the same convention
# (see web.strafe.chat/src/lib/emoji/providers.ts):
#   twemoji   <lowercase codepoints, '-' separated>.svg     (jdecked fork)
#   noto      emoji_u<lowercase codepoints, '_' separated>.svg
#   openmoji  <UPPERCASE codepoints, '-' separated>.svg      (colour SVGs)
#
# Licences (the sets are open, but two require visible attribution - see the web client's
# EMOJI_ATTRIBUTIONS and docs/CREDITS.md): twemoji graphics CC-BY 4.0, noto Apache-2.0,
# openmoji CC-BY-SA 4.0. The upstream LICENSE files are copied in beside each set.
set -euo pipefail

# Pinned versions. Bump these (and the web client's EMOJI_ATTRIBUTIONS / CREDITS.md) together
# when upgrading a set; changing VERSION makes nebula re-seed on its next start.
TWEMOJI_REF="${TWEMOJI_REF:-v16.0.1}"
OPENMOJI_VER="${OPENMOJI_VER:-15.1.0}"
# noto-emoji publishes no asset release tags; pin a commit for a reproducible build. `main`
# is the moving default - override NOTO_REF with a 40-char commit SHA in CI/release builds.
NOTO_REF="${NOTO_REF:-main}"

OUT_DIR="${1:-./emoji}"

log() { printf 'fetch-emoji: %s\n' "$*" >&2; }
need() { command -v "$1" >/dev/null 2>&1 || { log "missing required tool: $1"; exit 1; }; }
need curl
need tar

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# download URL -> local tarball
dl() {
  local url="$1" out="$2"
  log "downloading $url"
  curl -fsSL "$url" -o "$out"
}

mkdir -p "$OUT_DIR"

# ---- twemoji (jdecked fork) --------------------------------------------------------------
log "twemoji $TWEMOJI_REF"
dl "https://codeload.github.com/jdecked/twemoji/tar.gz/refs/tags/${TWEMOJI_REF}" "$work/twemoji.tgz"
tar -xzf "$work/twemoji.tgz" -C "$work"
tw_src="$(find "$work" -maxdepth 1 -type d -name 'twemoji-*' | head -1)"
rm -rf "$OUT_DIR/twemoji"; mkdir -p "$OUT_DIR/twemoji"
cp "$tw_src"/assets/svg/*.svg "$OUT_DIR/twemoji/"
# The *graphics* are CC-BY 4.0 (the code is MIT); ship the graphics licence.
cp "$tw_src"/LICENSE-GRAPHICS "$OUT_DIR/twemoji/LICENSE"

# ---- noto emoji --------------------------------------------------------------------------
log "noto $NOTO_REF"
if printf '%s' "$NOTO_REF" | grep -qE '^[0-9a-f]{40}$'; then
  noto_url="https://codeload.github.com/googlefonts/noto-emoji/tar.gz/${NOTO_REF}"
else
  noto_url="https://codeload.github.com/googlefonts/noto-emoji/tar.gz/refs/heads/${NOTO_REF}"
fi
dl "$noto_url" "$work/noto.tgz"
tar -xzf "$work/noto.tgz" -C "$work"
noto_src="$(find "$work" -maxdepth 1 -type d -name 'noto-emoji-*' | head -1)"
rm -rf "$OUT_DIR/noto"; mkdir -p "$OUT_DIR/noto"
# Upstream moved the SVGs under 2D/svg (they used to live in svg/); this is the current path.
cp "$noto_src"/2D/svg/emoji_u*.svg "$OUT_DIR/noto/"
cp "$noto_src"/LICENSE "$OUT_DIR/noto/LICENSE"

# ---- openmoji (colour) -------------------------------------------------------------------
log "openmoji $OPENMOJI_VER"
dl "https://registry.npmjs.org/openmoji/-/openmoji-${OPENMOJI_VER}.tgz" "$work/openmoji.tgz"
tar -xzf "$work/openmoji.tgz" -C "$work"          # npm tarballs extract under package/
rm -rf "$OUT_DIR/openmoji"; mkdir -p "$OUT_DIR/openmoji"
cp "$work"/package/color/svg/*.svg "$OUT_DIR/openmoji/"
cp "$work"/package/LICENSE.txt "$OUT_DIR/openmoji/LICENSE"

# ---- version marker ----------------------------------------------------------------------
# nebula stores this string and re-seeds whenever it changes, so bumping any ref above
# refreshes the served set on the next start.
cat > "$OUT_DIR/VERSION" <<EOF
twemoji=${TWEMOJI_REF}
noto=${NOTO_REF}
openmoji=${OPENMOJI_VER}
EOF

count() { find "$OUT_DIR/$1" -name '*.svg' | wc -l | tr -d ' '; }
log "done -> $OUT_DIR (twemoji=$(count twemoji) noto=$(count noto) openmoji=$(count openmoji) svg)"
