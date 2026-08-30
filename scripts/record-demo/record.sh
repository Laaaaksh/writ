#!/usr/bin/env bash
# Builds writ, seeds a throwaway demo repo, records the real CLI with VHS,
# and converts the capture into docs/assets/demo.{mp4,gif}.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v vhs >/dev/null 2>&1; then
	echo "vhs not found. Install it with: brew install vhs" >&2
	exit 1
fi
if ! command -v ffmpeg >/dev/null 2>&1; then
	echo "ffmpeg not found. Install it with: brew install ffmpeg" >&2
	exit 1
fi

BIN_DIR="scripts/record-demo/.bin"
# Kept short and at the repo root (not nested under scripts/record-demo/)
# so writ's printed absolute paths (e.g. "writ approved: <path>") stay
# within the recorded terminal's column width instead of wrapping.
REPO_DIR=".demo-repo"

rm -rf "$BIN_DIR" "$REPO_DIR"
mkdir -p "$BIN_DIR" "$REPO_DIR"

echo "building writ..."
go build -o "$BIN_DIR/writ" ./cmd/writ

echo "seeding throwaway demo repo at $REPO_DIR..."
(
	cd "$REPO_DIR"
	git init -q -b master
	git config user.email "demo@example.invalid"
	git config user.name "writ demo"

	mkdir -p internal/webhook internal/other
	cat >internal/webhook/sender.go <<'EOF'
package webhook

func Send(url string) error {
	return nil
}
EOF
	cat >internal/other/thing.go <<'EOF'
package other

func Thing() int {
	return 1
}
EOF
	cat >go.mod <<'EOF'
module demo

go 1.22
EOF
	cat >writ-proposal.toml <<'EOF'
id = "writ-1"
intent = "add retry to the webhook sender"
base = "master"
created = 2026-08-30T00:00:00Z
scope = ["internal/webhook/**"]

[[criteria]]
id = "retries-on-5xx"
text = "a 5xx response is retried with backoff"

[verify]
command = "go build ./..."
EOF

	git add -A
	git commit -q -m "initial"
)

mkdir -p docs/assets
rm -f docs/assets/demo.mp4 docs/assets/demo.gif

echo "recording with vhs..."
vhs scripts/record-demo/demo.tape

echo "converting to gif (palette-quantized, 960px, 12fps)..."
PALETTE="$(mktemp -t writ-demo-palette).png"
ffmpeg -y -i docs/assets/demo.mp4 -vf "fps=12,scale=960:-1:flags=lanczos,palettegen" "$PALETTE" -loglevel error
ffmpeg -y -i docs/assets/demo.mp4 -i "$PALETTE" \
	-filter_complex "fps=12,scale=960:-1:flags=lanczos[x];[x][1:v]paletteuse" \
	docs/assets/demo.gif -loglevel error
rm -f "$PALETTE"

echo
echo "done:"
ls -lh docs/assets/demo.mp4 docs/assets/demo.gif
echo
echo "verify with:"
echo "  ffprobe -v error -select_streams v:0 -show_entries stream=width,height,nb_frames,codec_name,pix_fmt -of default=noprint_wrappers=1 docs/assets/demo.mp4"
echo "  ffprobe -v error -select_streams v:0 -show_entries stream=width,height,nb_frames -of default=noprint_wrappers=1 docs/assets/demo.gif"

rm -rf "$BIN_DIR" "$REPO_DIR"
