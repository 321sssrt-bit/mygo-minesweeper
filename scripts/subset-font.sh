#!/bin/sh
# Subsets LXGW WenKai (OFL) to the characters the game's source uses and
# writes the result to resources/fonts. Rerun after changing any copy;
# TestEmbeddedFontCoversCopy fails when a character is missing.
#
# Needs: go, curl, shasum, pyftsubset (pip install fonttools).
set -eu

VERSION=v1.530
CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/mygo-minesweeper/fonts/$VERSION"
OUT=resources/fonts
REGULAR_SHA=2d73eea94ad808999d90c0000c239a84a33b09d4bc6ab6c5b8bbf1eda8a7fc8b
MEDIUM_SHA=f4b8a34d1f838fe411540cb7c97b1c384ac9738a876c4104500bc42508b2312a

cd "$(dirname "$0")/.."
mkdir -p "$CACHE" "$OUT"

fetch() { # name sha
	if [ ! -f "$CACHE/$1" ]; then
		curl -sSfL -o "$CACHE/$1.part" "https://github.com/lxgw/LxgwWenKai/releases/download/$VERSION/$1"
		mv "$CACHE/$1.part" "$CACHE/$1"
	fi
	echo "$2  $CACHE/$1" | shasum -a 256 -c - >/dev/null
}
fetch LXGWWenKai-Regular.ttf "$REGULAR_SHA"
fetch LXGWWenKai-Medium.ttf "$MEDIUM_SHA"

GLYPHS="$CACHE/glyphs.txt"
go run ./tools/glyphs > "$GLYPHS"

for weight in Regular Medium; do
	pyftsubset "$CACHE/LXGWWenKai-$weight.ttf" \
		--text-file="$GLYPHS" \
		--layout-features='kern,liga,locl,ccmp' \
		--no-hinting --desubroutinize \
		--output-file="$OUT/LXGWWenKai-$weight.subset.ttf"
done
ls -l "$OUT"
