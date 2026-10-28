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
# Claude Code itself is installed, or the git and shell settings of the runner's own home, and none
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
deny_write+=("$@")
if [ -n "${GITHUB_WORKSPACE:-}" ] && [ "$GITHUB_WORKSPACE" != "$tree" ]; then
  deny_write+=("$GITHUB_WORKSPACE/.git")
fi
if [ -n "${RUNNER_TOOL_CACHE:-}" ]; then deny_write+=("$RUNNER_TOOL_CACHE"); fi
if [ -n "${RUNNER_TEMP:-}" ]; then
  for dir in "$(dirname "$RUNNER_TEMP")/_actions" "$(dirname "$RUNNER_TEMP")/_tool"; do
    if [ -d "$dir" ]; then deny_write+=("$dir"); fi
  done
fi
for file in .gitconfig .config .bashrc .bash_profile .profile .local .npmrc .ssh; do
  deny_write+=("$HOME/$file")
done

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
