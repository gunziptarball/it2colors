# it2colors

[![CI](https://github.com/gunziptarball/it2colors/actions/workflows/ci.yml/badge.svg)](https://github.com/gunziptarball/it2colors/actions/workflows/ci.yml)

Pick an iTerm2 color scheme and apply it to the running terminal — without touching iTerm2's preferences.

A small Go CLI that reads `.itermcolors` files from the [mbadolato/iterm2-color-schemes](https://github.com/mbadolato/iterm2-color-schemes) archive and applies them on the fly via OSC escape sequences. Drop it in your `.bashrc` / `.zshrc` and every new shell starts with a different scheme.

macOS + iTerm2 only.

## Install

Get the schemes archive (one-time):

```sh
curl -L https://github.com/mbadolato/iTerm2-Color-Schemes/archive/refs/heads/master.zip -o /tmp/schemes.zip
unzip /tmp/schemes.zip -d ~/Downloads/
```

Build the binary:

```sh
go install github.com/gunziptarball/it2colors@latest
```

…or from a checkout:

```sh
go build -o ~/.local/bin/it2colors .
```

## Usage

```sh
it2colors                        # random scheme
it2colors Adventure              # named scheme
it2colors -l                     # list all available schemes
it2colors --hue 30               # random scheme, hue-rotated 30°
it2colors --lightness -0.2       # darken every color by 0.2 (good for bright themes)
it2colors --saturation 0.7       # desaturate to 70%
it2colors -c --hue 30            # tint the current scheme in place by 30°
it2colors -q                     # apply quietly (no "Applied scheme:" status line)
it2colors -p                     # apply, then print a color test table
it2colors --help                 # full flag reference
```

## Shell integration

Add to `.bashrc` / `.zshrc`:

```bash
if [ -z "$CLAUDECODE" ] && [ -t 1 ] && command -v it2colors >/dev/null; then
  eval "$(it2colors -r --eval)"
fi
```

Each new interactive shell picks a random scheme. The guards:
- `[ -z "$CLAUDECODE" ]` — skips Claude Code's subshells, which source `.zshrc` before running commands and would otherwise apply a new random scheme on every tool call.
- `[ -t 1 ]` — skips non-interactive shells (cron, `scp`, etc.) where `/dev/tty` doesn't exist.

`--eval` exports `IT2COLORS_SCHEME=<name>` so subsequent calls like `it2colors -c --hue 30` know which scheme is active in *this* shell — handy when several iTerm tabs are running different schemes.

## Shell completion

Tab-complete scheme names by sourcing the completion script for your shell:

```bash
# zsh — add to .zshrc
source <(it2colors --completion zsh)

# bash — add to .bashrc
source <(it2colors --completion bash)

# fish — add to config.fish
it2colors --completion fish | source
```

## Schemes directory

`it2colors` resolves the schemes archive in this order:

1. `--schemes-dir` flag.
2. `$ITERM2_COLOR_SCHEMES_PATH`.
3. Newest `~/Downloads/mbadolato-iTerm2-Color-Schemes-*` directory.

The directory must contain a `schemes/` subdirectory with `.itermcolors` files inside it (the layout you get from extracting the archive).

## Color transforms

All three flags operate in CIE HCL space and can be combined freely:

| Flag | Effect |
|------|--------|
| `--hue N` | Rotate every color's hue by N degrees. Preserves perceived lightness so contrast stays readable. |
| `--lightness N` | Shift perceived lightness by N (range ≈ −1 to +1). Negative darkens — useful when a randomly chosen theme is painfully bright. |
| `--saturation F` | Scale chroma by factor F (1.0 = no change, 0 = fully desaturate, 2.0 = double vividness). |

Achromatic colors (pure black/white/gray) are unaffected by `--hue` and `--saturation` but do respond to `--lightness`.

Combine with `-c` to adjust the active scheme in place:

```sh
it2colors -c --hue 30            # warmer
it2colors -c --lightness -0.2    # darken a bright theme
it2colors -c --saturation 0.5    # mute a garish theme
```

## Preview

`-p` / `--preview` prints the same foreground × background color matrix used to capture the per-scheme PNGs in the upstream archive's `screenshots/` directory — handy for quickly seeing what a scheme looks like:

```sh
it2colors Adventure -p           # apply Adventure and show the table
it2colors -c -p                  # show the table for the current scheme
it2colors -c --hue 30 -p         # tint the current scheme and preview the result
```

## Favorites and yuck list

Build a curated pool over time:

```sh
it2colors -f             # favorite the current scheme (--favorite)
it2colors --unfavorite   # remove it from favorites
it2colors --yuck         # blacklist it — never picks it again
it2colors --yuck -r      # blacklist and immediately move to a new scheme
```

Once `~/.it2colors_favorites` is non-empty, `-r` draws only from that pool. Use `--all` to temporarily override and pick from the full archive (yuck list still applies).

The yuck list is stored in `~/.it2colors_yuck` — a plain sorted text file you can edit directly.

## Saved aliases and scheme defaults

Bake a tweaked theme into a reusable name with `--save`, or attach HSV defaults to a base scheme so they apply automatically every time you pick it.

```sh
# Save Adventure with hue +30° and a small darken under a new name:
it2colors Adventure --hue 30 --lightness -0.1 --save warm-adventure

# Invoke the alias like any scheme:
it2colors warm-adventure

# CLI flags override the saved baseline component-wise — here, hue is
# taken from the CLI but lightness still comes from the alias:
it2colors warm-adventure --hue 0

# Always-darken Adventure when it's picked plainly or by -r:
it2colors Adventure --save-defaults --lightness -0.1

# Clear those defaults later:
it2colors Adventure --save-defaults

# Browse and clean up:
it2colors --list-aliases
it2colors --delete-alias warm-adventure

# Include aliases in the random pool:
it2colors -r --include-aliases
```

Aliases are stored in `~/.it2colors_settings.json` alongside scheme defaults. A JSON Schema is checked into the repo at [`schema/it2colors-settings.schema.json`](schema/it2colors-settings.schema.json) for editor validation — VS Code, JetBrains, and friends will autocomplete and lint hand-edits.

## History

Every applied scheme is appended to `~/.it2colors_history`:

```
2026-04-26T20:06:11-04:00	Adventure
```

The last 20 entries are excluded from random selection so you don't see the same scheme twice in quick succession. (This exclusion is skipped when picking from a small favorites pool.)

## License

[MIT](LICENSE).
