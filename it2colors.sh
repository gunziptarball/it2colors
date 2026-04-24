#!/usr/bin/env bash
#
# Dynamically apply an iTerm2 color scheme (or a random one).
#
# Reads schemes from the mbadolato/iTerm2-Color-Schemes archive, available
# at https://iterm2colorschemes.com/. If $ITERM2_COLOR_SCHEMES_PATH is not
# set, picks the most recent mbadolato-iTerm2-Color-Schemes-* directory in
# ~/Downloads.
#
# Usage:
#   it2colors.sh [-v] [-r] [<profile name>]
#     -v             verbose: also run screenshotTable.sh
#     -r             pick a random scheme (default when no profile is given)
#     <profile name> name of an .itermcolors file (without the extension)
#
# Examples:
#   it2colors.sh Adventure
#   it2colors.sh -r

set -uo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: it2colors.sh [-v] [-r] [<profile name>]
  -v             verbose: also run screenshotTable.sh
  -r             pick a random scheme (default when no profile is given)
  <profile name> name of an .itermcolors file (without the extension)
EOF
  exit 2
}

# shellcheck disable=SC2010
default_schemes_dir() {
  local match
  match="$(ls -t1 "$HOME/Downloads" 2>/dev/null \
    | grep -E '^mbadolato-iTerm2-Color-Schemes-[a-f0-9]+$' \
    | head -n 1)"
  [[ -n "$match" ]] && echo "$HOME/Downloads/$match"
}

schemes_path="${ITERM2_COLOR_SCHEMES_PATH:-$(default_schemes_dir)}"

verbose=0
random=0
while getopts "vr" opt; do
  case "$opt" in
    v) verbose=1 ;;
    r) random=1 ;;
    *) usage ;;
  esac
done
shift $((OPTIND - 1))

choice="${1:-}"
[[ -z "$choice" ]] && random=1

if [[ ! -d "$schemes_path" ]]; then
  echo "Schemes directory not found: $schemes_path" >&2
  exit 1
fi

if (( random )); then
  profile="$(ls "$schemes_path/schemes" | sort -R | head -n 1)"
else
  profile="$choice.itermcolors"
fi

profile_path="$schemes_path/schemes/$profile"

if [[ ! -f "$profile_path" ]]; then
  echo "Profile not found: $profile_path" >&2
  exit 1
fi

echo "Applying colors from $profile_path"
"$schemes_path/tools/preview.rb" "$profile_path"

if (( verbose )); then
  "$schemes_path/tools/screenshotTable.sh"
fi

echo "$profile" >> "$HOME/.it2colors_history"
export ITERM2_CURRENT_COLOR_SCHEME="$profile_path"
