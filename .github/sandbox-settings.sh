#!/bin/bash
# Writes the user settings of one Claude session's own home, which --setting-sources user loads and
# nothing in the tree can change, so the commands the session runs do so in Claude Code's sandbox
# (#131). The workflows run main's copy, copied out of the tree before the session starts.
#
# Usage: sandbox-settings.sh HOME TREE KIND [PATH...], run before HOME is changed for the session,
# where HOME is the session's own home, TREE the working tree it runs in, KIND either workspace, for
# a session whose tree's .git host git reads after it, or clone, for the rebase's session, which runs
# git itself in a clone of its own that host git never touches again, and each PATH one more that no
# command may write, such as another session's folder.
#
# The sandbox is on, Claude Code refuses to start without it, and no command may retry outside it.
# CLAUDE_CODE_SUBPROCESS_ENV_SCRUB, which every session sets, runs each command in a PID namespace
# of its own, so it can't read Claude Code's environment through /proc, and adds its own list of
# paths no command may write, beside writes allowed anywhere under /home, /tmp, /opt and the like,
# which no setting can narrow. So on top of the scrub's list, no command may write the workspace's
# .git, nor the tree's for KIND workspace, the runner's own files, its actions, its tool cache, where
# Claude Code itself is installed, any folder on PATH or Go's bin, or the git and shell settings of
# the runner's own home, and none
# may read the runner's files: the scrub keys its own list to HOME, which is the session's here. No
# host is allowed, and no prompt is given for one: a session's Go modules and packages are fetched
# before it starts.
#
# Needs jq.
set -euo pipefail

home="$1"
tree="$2"
kind="$3"
shift 3
case "$kind" in
  workspace) deny_write=("$tree/.git") ;;
  clone) deny_write=() ;;
  *) echo "The kind must be workspace or clone: $kind" >&2; exit 1 ;;
esac
extra=("$@")
if [ -n "${GITHUB_WORKSPACE:-}" ] && [ "$GITHUB_WORKSPACE" != "$tree" ]; then
  deny_write+=("$GITHUB_WORKSPACE/.git")
fi
if [ -n "${RUNNER_TOOL_CACHE:-}" ]; then deny_write+=("$RUNNER_TOOL_CACHE"); fi
if [ -n "${RUNNER_TEMP:-}" ]; then
  for dir in "$(dirname "$RUNNER_TEMP")/_actions" "$(dirname "$RUNNER_TEMP")/_tool"; do
    if [ -d "$dir" ]; then deny_write+=("$dir"); fi
  done
fi
# A path the sandbox protects must exist, or bubblewrap may refuse to start every command, and a
# path left unprotected because it was missing is one a command could create for a later step to
# find. So each is made, empty, before it's denied; one this user can't make, a command can't either.
deny_made() {
  local kind="$1" path="$2"
  if [ "$kind" = dir ]; then mkdir -p "$path" 2> /dev/null || true; else
    [ -e "$path" ] || { mkdir -p "$(dirname "$path")" 2> /dev/null && : > "$path"; } 2> /dev/null || true
  fi
  if [ -e "$path" ]; then deny_write+=("$path"); fi
}
for path in "${extra[@]}"; do
  if [ -e "$path" ]; then deny_write+=("$path"); else deny_made dir "$path"; fi
done
for file in .gitconfig .bashrc .bash_profile .profile .npmrc; do deny_made file "$HOME/$file"; done
for dir in .config .local .ssh; do deny_made dir "$HOME/$dir"; done
# Every folder on PATH, and Go's own bin, so a command can't put a program where a later step would
# find it, whatever list of its own the scrub keeps.
IFS=: read -r -a path_dirs <<< "${PATH:-}"
for dir in "${path_dirs[@]}"; do
  case "$dir" in /*) deny_made dir "${dir%/}" ;; esac
done
if command -v go > /dev/null; then
  gopath="$(go env GOPATH 2> /dev/null || true)"
  IFS=: read -r -a gopaths <<< "$gopath"
  for dir in "${gopaths[@]}"; do
    case "$dir" in /*) deny_made dir "$dir/bin" ;; esac
  done
fi

# The runner's own files: the folder of the Runner.Worker this step runs under, found by walking up
# the processes, and where hosted runners keep it under the runner's home.
runner=()
pid=$$
while [ "$pid" -gt 1 ] 2> /dev/null; do
  exe="$(readlink -f "/proc/$pid/exe" 2> /dev/null || true)"
  case "$exe" in
    */bin/Runner.Worker|*/bin/Runner.Listener) runner+=("$(dirname "$(dirname "$exe")")"); break ;;
  esac
  pid="$(awk '{ print $4 }' "/proc/$pid/stat" 2> /dev/null || echo 1)"
done
for dir in "$HOME/runners" "$HOME/actions-runner"; do
  if [ -d "$dir" ]; then runner+=("$dir"); fi
done

list() { jq -nc '$ARGS.positional | unique' --args "$@"; }
mkdir -p "$home/.claude"
jq -n --argjson write "$(list "${deny_write[@]}" "${runner[@]}")" --argjson read "$(list "${runner[@]}")" '
  {sandbox: {
    enabled: true,
    failIfUnavailable: true,
    allowUnsandboxedCommands: false,
    autoAllowBashIfSandboxed: false,
    filesystem: {denyWrite: $write, denyRead: $read},
    network: {allowedDomains: [], strictAllowlist: true}
  }}' > "$home/.claude/settings.json"
