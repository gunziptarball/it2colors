#!/usr/bin/env bash
 # Dynamically change an iTerm2 session's color scheme (or choose a random one)
 # Requires downloading from https://iterm2colorschemes.com/
 # By default script will look in your Downloads directory under an expected name
 # Usage: iterm2colors.sh <profile name | --random | --yuck>
 # Example: iterm2colors.sh --random
 # iterm2colors.sh Adventure
 
 schemes_path="${ITERM2_COLOR_SCHEMES_PATH:-$HOME/Downloads/$(
   # shellcheck disable=SC2010
   ls -t1 "$HOME/Downloads" |
     grep -E '^mbadolato-iTerm2-Color-Schemes-[a-f0-9]+$' |
     head -n 1
 )}"
 
 OPTIND=1
 
 verbose=0
 while getopts "vr" opt; do
   case "$opt" in
     v)
       verbose=1
       ;;
     r)
       choice="$opt"
       ;;
     *) echo "Extra: $opt"
       ;;
   esac
 done
 
 shift $((OPTIND-1))
 
 [ "${1:-}" = "--" ] && shift
 
 choice="$1"
 
 if [ -z "$choice" ]; then
   choice="--random"
 fi
 
 if [ ! -d "$schemes_path" ]; then
   echo "$schemes_path not a directory or does not exist."
   exit 1
 fi
 
 if [ "$choice" == "--random" ]; then
   # shellcheck disable=SC2012
   if [ -z "$group" ]; then
     schemes_path="${schemes_path}/${group}"
   fi
   profile="$(ls "$schemes_path/schemes" | sort -R | head -n 1)"
 else
   profile="$choice.itermcolors"
 fi
 
 profile_path="$schemes_path/schemes/$profile"
 
 if [ ! -f "$profile_path" ]; then
   echo "Profile $profile_path does not exist."
   exit 1
 fi
 
 echo "Applying colors from $profile_path"
 
 "$schemes_path/tools/preview.rb" "$profile_path" | cat
 
 if [ $verbose == 1 ]; then
   "$schemes_path/tools/screenshotTable.sh"
 fi
 
 echo "$profile" >>"$HOME/.it2colors_history"
 
 export ITERM2_CURRENT_COLOR_SCHEME=$profile_path

