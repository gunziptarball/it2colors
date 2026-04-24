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
#   it2colors.sh [-v] [-r] [-X] [<profile name>]
#     -v             verbose: also run screenshotTable.sh
#     -r             pick a random scheme (default when no profile is given)
#     -X             "yuck": move the named (or current) scheme out of
#                    circulation into <schemes_path>/yuck/
#     <profile name> name of an .itermcolors file (without the extension)
#
# Examples:
#   it2colors.sh Adventure
#   it2colors.sh -r
#   it2colors.sh -X            # yuck the currently applied scheme

set -eu

usage() {
  cat >&2 <<'EOF'
Usage: it2colors.sh [-v] [-r] [-X] [<profile name>]
  -v             verbose: also run screenshotTable.sh
  -r             pick a random scheme (default when no profile is given)
  -X             yuck: move the named (or current) scheme out of circulation
  <profile name> name of an .itermcolors file (without the extension)
EOF
  exit 2
}

# shellcheck disable=SC2010
default_schemes_dir() {
  local match
  match="$(ls -t1 "$HOME/Downloads" 2>/dev/null \
    | grep -E '^mbadolato-iTerm2-Color-Schemes-[a-f0-9]+$' \
    | head -n 1)" || return 0
  [[ -n "$match" ]] && echo "$HOME/Downloads/$match"
  return 0
}

schemes_path="${ITERM2_COLOR_SCHEMES_PATH:-$(default_schemes_dir)}"
current_file="$HOME/.it2colors_current"
history_file="$HOME/.it2colors_history"

current_scheme=""
[[ -f "$current_file" ]] && current_scheme="$(cat "$current_file")"

verbose=0
random=0
yuck=0
while getopts "vrX" opt; do
  case "$opt" in
    v) verbose=1 ;;
    r) random=1 ;;
    X) yuck=1 ;;
    *) usage ;;
  esac
done
shift $((OPTIND - 1))

choice="${1:-}"

if [[ ! -d "$schemes_path" ]]; then
  echo "Schemes directory not found: $schemes_path" >&2
  exit 1
fi

if (( yuck )); then
  # Default to yucking whatever is currently applied.
  [[ -z "$choice" ]] && choice="${current_scheme%.itermcolors}"
  if [[ -z "$choice" ]]; then
    printf '\033[0;1;31mPlease specify a scheme to "yuck out"\033[0m\n' >&2
    exit 1
  fi
  echo "Didn't like that one, eh?  Try another."
  mkdir -p "$schemes_path/yuck"
  mv -i "$schemes_path/schemes/$choice.itermcolors" "$schemes_path/yuck/"
  exit 0
fi

[[ -z "$choice" ]] && random=1

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

echo "$profile" >> "$history_file"
echo "$profile" > "$current_file"
export ITERM2_CURRENT_COLOR_SCHEME="$profile_path"
