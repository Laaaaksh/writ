# Recording the demo

`record.sh` builds the real `writ` binary, seeds a throwaway git repo, drives
that repo through the actual propose -> approve -> implement -> attest ->
status loop under [VHS](https://github.com/charmbracelet/vhs), and converts
the capture into the assets the README embeds. Nothing in the recording is
staged or hand-edited: `demo.tape` types real commands into a real shell
running the real binary.

## Run it

```
brew install vhs ffmpeg   # if not already installed
make demo                 # or: scripts/record-demo/record.sh
```

This writes:

- `docs/assets/demo.mp4` - the raw capture (1280px, H.264, yuv420p)
- `docs/assets/demo.gif` - a palette-quantized conversion (960px, 12fps) for
  the README, kept under 10 MB

## How it fits together

- `record.sh` builds `writ` into `scripts/record-demo/.bin/` (gitignored),
  seeds `.demo-repo/` at the repo root (gitignored) with a two-package Go
  module and a writ proposal, runs `vhs scripts/record-demo/demo.tape`, then
  shells out to `ffmpeg` for the GIF conversion. Both scratch directories are
  removed again when it finishes. `.demo-repo` is kept short and at the repo
  root (not nested under `scripts/record-demo/`) so writ's printed absolute
  paths - `writ approved: <path>` - stay within the recorded terminal's
  column width instead of wrapping.
- `demo.tape` is the actual recording script, committed so the capture is
  reviewable and reproducible from the diff. It types the CLI commands live:
  propose from a file, approve `--yes`, an in-scope edit to
  `internal/webhook/sender.go` that gets attested and shows a clean
  `writ status`, then an out-of-scope edit to `internal/other/thing.go` that
  `writ status` flags as drift.

## Verify the output

```
ffprobe -v error -select_streams v:0 \
  -show_entries stream=width,height,nb_frames,codec_name,pix_fmt \
  -of default=noprint_wrappers=1 docs/assets/demo.mp4

ffprobe -v error -select_streams v:0 \
  -show_entries stream=width,height,nb_frames \
  -of default=noprint_wrappers=1 docs/assets/demo.gif
```

A useful capture has hundreds of frames at the resolutions above, not a
handful - check `nb_frames` and open the GIF to confirm it plays the loop.

## Re-recording

Re-run `make demo` any time the CLI's output format changes; the tape
re-seeds its own throwaway repo each run, so nothing from a previous
recording carries over.
