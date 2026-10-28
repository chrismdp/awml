#!/bin/bash
# Writes the user settings of one Claude session's own home, which --setting-sources user loads and
# nothing in the tree can change, so the commands the session runs do so in Claude Code's sandbox
# (#131). The workflows run main's copy, copied out of the tree before the session starts.
#
# Usage: sandbox-settings.sh HOME TREE KIND, run before HOME is changed for the session, where HOME
# is the session's own home, TREE the working tree it runs in, and KIND either builds, for a session
# whose commands build and test the change, or reads, for one that runs no commands at all.
#
# The sandbox is on, Claude Code refuses to start without it, and no command may retry outside it.
# CLAUDE_CODE_SUBPROCESS_ENV_SCRUB, which every session sets, runs each command in a PID namespace
# of its own, so it can't read Claude Code's environment through /proc, and adds its own list of
# paths no command may write. On top of that, no command may write .git, of the tree or of the
# workspace, or read or write the runner's own files: the scrub keys its list to HOME, which is the
# session's here, not the runner's. The network is refused but for the hosts a build's commands need,
# the Go module proxy, its checksum database and npm's registry, with no prompt for any other.
#
# Needs jq.
set -euo pipefail

home="$1"
tree="$2"
kind="$3"
case "$kind" in
  builds) domains=(proxy.golang.org sum.golang.org registry.npmjs.org) ;;
  reads) domains=() ;;
  *) echo "The kind must be builds or reads: $kind" >&2; exit 1 ;;
esac

deny_write=("$tree/.git")
if [ -n "${GITHUB_WORKSPACE:-}" ] && [ "$GITHUB_WORKSPACE" != "$tree" ]; then
  deny_write+=("$GITHUB_WORKSPACE/.git")
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
if [ -n "${RUNNER_TEMP:-}" ] && [ -d "$(dirname "$RUNNER_TEMP")/_actions" ]; then
  deny_write+=("$(dirname "$RUNNER_TEMP")/_actions")
fi

list() { jq -nc '$ARGS.positional | unique' --args "$@"; }
mkdir -p "$home/.claude"
jq -n --argjson write "$(list "${deny_write[@]}" "${runner[@]}")" --argjson read "$(list "${runner[@]}")" \
  --argjson domains "$(jq -nc '$ARGS.positional' --args "${domains[@]}")" '
  {sandbox: {
    enabled: true,
    failIfUnavailable: true,
    allowUnsandboxedCommands: false,
    autoAllowBashIfSandboxed: false,
    filesystem: {denyWrite: $write, denyRead: $read},
    network: {allowedDomains: $domains, strictAllowlist: true}
  }}' > "$home/.claude/settings.json"
