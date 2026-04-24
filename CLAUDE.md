# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository

A single bash script, `it2colors.sh`, that applies an iTerm2 color scheme to the current session (a named one, or a random one).

## External dependency

The script does not ship the color schemes. It expects the [mbadolato/iTerm2-Color-Schemes](https://iterm2colorschemes.com/) archive to be unpacked on disk, and shells out to two tools inside it:

- `<schemes_path>/tools/preview.rb` — emits the OSC escape sequence that actually changes the terminal's colors. Calling this is the script's whole reason to exist; it is not optional cosmetic output.
- `<schemes_path>/tools/screenshotTable.sh` — only invoked under `-v`.

`schemes_path` resolution: `$ITERM2_COLOR_SCHEMES_PATH` if set, else the most recent `~/Downloads/mbadolato-iTerm2-Color-Schemes-<sha>/` directory. Profiles live at `<schemes_path>/schemes/<name>.itermcolors`.

## Side effects worth knowing

- Appends the chosen profile filename to `~/.it2colors_history` (full append-only log).
- Overwrites `~/.it2colors_current` with the just-applied profile filename. The `-X` ("yuck") flag reads this file to decide what to move out of circulation when no name is passed — keep it as a single-line overwrite, not an append.
- Exports `ITERM2_CURRENT_COLOR_SCHEME` — only useful if the script is *sourced* (e.g. `source it2colors.sh -r`), not executed as a subprocess. Preserve this when refactoring.

## The `-X` flag

`-X [<name>]` moves a scheme into `<schemes_path>/yuck/` so the random picker stops landing on it. With no name, it yucks whatever `~/.it2colors_current` says is currently applied. Uses `mv -i` so the user gets a confirmation prompt if a yucked file with the same name already exists.

## Installed copy

The script is also installed at `~/.local/bin/it2colors` (no `.sh` extension) and is the version the user actually runs. The two copies are not symlinked — when changing behavior, update both files.

## Testing

There is no test suite. To verify changes, run the script in an iTerm2 session and confirm the colors change; `-v` plus a known profile name is the most informative smoke test.
