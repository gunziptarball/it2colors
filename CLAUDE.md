# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A small Go CLI, `it2colors`, that picks an iTerm2 color scheme from the [mbadolato/iterm2-color-schemes](https://github.com/mbadolato/iterm2-color-schemes) archive and applies it to the current iTerm2 session by writing OSC escape sequences to `/dev/tty`. Intended to be run from `.bashrc`/`.zshrc` so every new shell launches with a different scheme.

MVP target: macOS + iTerm2 only.

## Layout

Single-package, single-file CLI. Resist splitting into packages until there's a real reason — the entire program is ~300 lines.

```
main.go         CLI parsing, schemes-dir resolution, plist parse, OSC emit, history append
main_test.go    plist parse + OSC render + slot mapping + clamp tests
testdata/       one real .itermcolors fixture
```

External deps:
- `howett.net/plist` — parses `.itermcolors` plist XML.
- `github.com/lucasb-eyer/go-colorful` — sRGB ↔ CIE HCL conversions for the `--hue` transform.
- `github.com/spf13/pflag` — GNU-style short/long flag pairs (e.g. `-r` / `--random`). Imported as `flag`, drop-in for stdlib `flag`.

Don't reach for a CLI framework (cobra/urfave) — pflag alone is sufficient.

## Build / test / install

```
go build -o it2colors .          # local build
go test ./...                    # run tests
go build -o ~/.local/bin/it2colors .   # install (replaces previous binary)
```

## OSC escape contract (the load-bearing detail)

For each color in a `.itermcolors` plist, write:

```
ESC ] P <slot> <rrggbb> ESC \
```

Slot mapping (in `slotFor` in main.go):
- ANSI 0–15 → hex digit `'0'..'9'`, `'a'..'f'`
- `Foreground Color` → `g`, `Background Color` → `h`, `Bold Color` → `i`, `Selection Color` → `j`, `Selected Text Color` → `k`, `Cursor Color` → `l`, `Cursor Text Color` → `m`

Other keys in the plist (`Tab Color`, `Underline Color`, `Link Color`, etc.) have no OSC P slot and are silently skipped — don't error on them.

Components are `<real>` 0.0–1.0; clamp via `min(255, int(v*256))` (matches the Ruby `tools/preview.rb` behavior — values can be slightly >1 in the wild).

The OSC bytes go to `/dev/tty`, not stdout. This is essential because of the `--eval` mode (below) — stdout is captured by the user's `eval`, but the escape sequences still need to reach the terminal.

## Stream discipline

Three streams, three jobs — keep them separate:
- `/dev/tty`: OSC escape bytes, plus the `--preview` color table (also terminal-display content, written to the same writer right after the OSC bytes).
- stdout: only the `export IT2COLORS_SCHEME=<name>` line under `--eval`. Stays empty otherwise so `eval "$(it2colors …)"` is safe.
- stderr: the `Applied scheme: <name>` status line (with `(hue +N°)` suffix when rotated) and any errors. `-q` suppresses the status line for scripts that find it noisy; errors still print.

## Color transforms: `--hue`, `--lightness`, `--saturation`

All three operate in CIE HCL space (polar form of CIE Lab) via `adjustColors()`, which runs once on the parsed `Profile` map between `loadProfile` and `applyProfile`.

- **`--hue N`** — rotates every color's hue by N degrees. Preserves perceived lightness, so foreground/background contrast is essentially unchanged.
- **`--lightness N`** — shifts every color's L component by N (range roughly -1 to +1). Negative darkens; positive brightens. Applies to achromatic grays too, so `--lightness -0.3` will dim a blinding-white background.
- **`--saturation F`** — multiplies every color's chroma by F (1.0 = no change, 0 = fully desaturate, 2.0 = double vividness).

Implementation notes:
- Achromatic colors (chroma below ~1e-4, or NaN hue) skip hue rotation and saturation scaling, but still receive the lightness shift.
- Out-of-gamut results are clamped via `colorful.Color.Clamped()`. This causes ~3–8° hue drift on highly saturated rotations, and slightly reduces the effective lightness delta on colors near the gamut boundary.
- CIE HCL over OKLCH: go-colorful ships HCL out of the box; the perceptual difference for these transforms isn't worth pulling in a second library. Revisit if doing contrast-aware or deduplication work.

## `--preview`: SGR test table

`-p` / `--preview` prints the classic foreground × background color matrix to `/dev/tty` after applying the scheme. The output format is a verbatim port of the upstream archive's `tools/screenshotTable.sh` (originally from the [Bash Prompt HOWTO](http://tldp.org/HOWTO/Bash-Prompt-HOWTO/x329.html)) — the same script used to capture the per-scheme PNGs in `screenshots/`. Reproducing that exact format is intentional: it's what users recognize when comparing against the upstream gallery.

The preview uses pure SGR escapes (`\x1b[31m` etc.), not the OSC palette directly. It looks colorful only because we just applied the OSC palette one statement earlier; that ordering matters.

## --eval mode and per-shell state

The "current scheme" is tracked **per-shell via env var**, not via a global file. Old prototypes wrote to `~/.it2colors_current`; that's deliberately gone — it's meaningless when several iTerm tabs are open.

Under `--eval`, the binary writes a single line to stdout:

```
export IT2COLORS_SCHEME=<name>
```

Intended `.bashrc`/`.zshrc` usage:

```bash
if [ -z "$CLAUDECODE" ] && [ -t 1 ] && command -v it2colors >/dev/null; then
  eval "$(it2colors -r --eval)"
fi
```

The guards matter:
- `[ -z "$CLAUDECODE" ]` — Claude Code sets `CLAUDECODE=1` and sources `.zshrc` before each shell command it runs. Without this guard, every Claude Code tool call applies a new random scheme to the active iTerm2 session.
- `[ -t 1 ]` — skips non-interactive shells (cron, scp). Without it, the script fails at `open /dev/tty` because there's no controlling terminal.

A future `--yuck` flag should read `$IT2COLORS_SCHEME` to decide what to move out of circulation — that's the whole point of the per-shell tracking. (An earlier bash prototype had a `-X` flag for this; it was dropped in the Go rewrite and not yet re-ported.)

`-c` / `--current` reads `$IT2COLORS_SCHEME` to re-apply (or transform) the active scheme — `it2colors -c --hue 30` tints the current theme in place. Errors with a `.bashrc` hint if the env var isn't set, since the feature is meaningless without the eval-style integration.

## Schemes directory resolution

In order:
1. `--schemes-dir` flag.
2. `$ITERM2_COLOR_SCHEMES_PATH`.
3. Newest `~/Downloads/mbadolato-iTerm2-Color-Schemes-*` directory (mtime sort).

A "valid" schemes dir contains a `schemes/` subdirectory.

## History file

Append-only `~/.it2colors_history`, one line per invocation:

```
2026-04-26T20:06:11-04:00<TAB>Adventure
```

Failure to append is non-fatal — emits a `warning:` to stderr and continues. The history is for the human, not for the program.

## Testing reality check

`/dev/tty` is unavailable in non-interactive shells (CI, subprocess invocations, etc.). The unit tests cover plist parsing, escape rendering against a fixture, slot mapping, and clamping — none of those need a TTY. End-to-end "did the colors actually change" verification has to happen in an iTerm2 window by hand.

## Installed copy

The user runs `~/.local/bin/it2colors`. The repo build is the source; reinstall after changes with `go build -o ~/.local/bin/it2colors .` (it'll overwrite cleanly since the existing file is now also a Go binary, not the old bash script).
