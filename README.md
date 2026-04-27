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
it2colors                # random scheme
it2colors Adventure      # named scheme
it2colors -l             # list all available schemes (or --list)
it2colors --hue 30       # random scheme, hue-rotated 30°
it2colors -c --hue 30    # tint the current scheme in place by 30°
it2colors -q             # apply quietly (no "Applied scheme:" status line)
it2colors --help         # full flag reference
```

## Shell integration

Add to `.bashrc` / `.zshrc`:

```bash
if [ -t 1 ] && command -v it2colors >/dev/null; then
  eval "$(it2colors -r --eval)"
fi
```

Each new interactive shell picks a random scheme. The `[ -t 1 ]` guard skips non-interactive shells (cron, `scp`, etc.) where `/dev/tty` doesn't exist.

`--eval` exports `IT2COLORS_SCHEME=<name>` so subsequent calls like `it2colors -c --hue 30` know which scheme is active in *this* shell — handy when several iTerm tabs are running different schemes.

## Schemes directory

`it2colors` resolves the schemes archive in this order:

1. `--schemes-dir` flag.
2. `$ITERM2_COLOR_SCHEMES_PATH`.
3. Newest `~/Downloads/mbadolato-iTerm2-Color-Schemes-*` directory.

The directory must contain a `schemes/` subdirectory with `.itermcolors` files inside it (the layout you get from extracting the archive).

## Hue rotation

`--hue N` rotates every color in the chosen scheme by N degrees in CIE HCL space before applying it. Because HCL preserves perceived lightness, foreground/background contrast is essentially unchanged and the result stays readable. Achromatic colors (pure black/white/gray) pass through untouched — rotating their (undefined) hue would tint them, which looks wrong.

Combine with `-c` to tint the active scheme without picking a new one:

```sh
it2colors -c --hue 30     # warmer
it2colors -c --hue -30    # cooler
```

## History

Every applied scheme is appended to `~/.it2colors_history`:

```
2026-04-26T20:06:11-04:00	Adventure
```

For your eyes only — the program never reads it back.

## License

[MIT](LICENSE).
