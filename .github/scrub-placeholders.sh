#!/bin/bash
# Claude Code's scrub denies commands writing paths in the tree a session runs in, and in the
# workspace, that could change what runs next. Where one doesn't exist, the sandbox mounts /dev/null
# or an empty folder there, and makes the mount point in the real tree: an empty file, or an empty
# folder for a path that holds another denied one. It leaves them there. This keeps them out of
# what host git reads after a session.
#
# Usage:
#   scrub-placeholders.sh before TREE STATE [standin]  records in STATE where under TREE a placeholder
#     may be made, and with standin puts a stand-in of its own in place of a missing package.json or
#     .git/commondir, since node refuses an empty package.json and git an empty commondir.
#   scrub-placeholders.sh after TREE STATE  removes each placeholder that is still empty, and each
#     stand-in that's unchanged, and fails if one holds anything, is a link, or a stand-in changed.
#     A .claude folder the session left is said in full and removed, whatever it holds. A
#     placeholder inside another goes first.
#   scrub-placeholders.sh exclude TREE  names each path in TREE's .git/info/exclude, so git add in a
#     session's own clone commits no placeholder.
#
# The workflows run main's copy, kept in the host's own folder, which no session can write.
set -euo pipefail

# The paths Claude Code 2.1.312 denies commands writing in the tree they run in: the scrub's, the
# sandbox's own at the tree's top, and the permission system's in .claude.
paths=(package.json package-lock.json yarn.lock pnpm-lock.yaml bunfig.toml .npmrc .yarnrc .yarnrc.yml
  .env .env.local .env.development .env.development.local .env.test .env.test.local
  .env.production .env.production.local node_modules/.bin scripts .github .gitmodules
  .gitconfig .bashrc .bash_profile .zshrc .zprofile .profile .ripgreprc .mcp.json .vscode .idea
  .claude .claude/settings.json .claude/settings.local.json .claude/commands .claude/agents
  .claude/hooks .claude/skills
  .git/hooks .git/config .git/config.lock .git/config.worktree .git/config.worktree.lock
  .git/commondir .git/worktrees .git/modules .git/info/exclude .git/glab-cli)
# The stand-ins: a package.json node reads as a typeless package, and a commondir naming the git
# directory itself.
declare -A standins=([package.json]='{"private": true}' [.git/commondir]='.')

mode="$1"
tree="$2"
case "$mode" in
  before)
    state="$3"
    mkdir -p "$(dirname "$state")"
    : > "$state"
    for path in "${paths[@]}"; do
      # The mount point is made at the first part of the path that's missing, and a later one, for
      # a path inside it, may be made in that one: every missing part is recorded.
      missing=false
      sofar=""
      IFS=/ read -r -a parts <<< "$path"
      for part in "${parts[@]}"; do
        sofar="${sofar:+$sofar/}$part"
        if [ "$missing" = true ] || { [ ! -e "$tree/$sofar" ] && [ ! -L "$tree/$sofar" ]; }; then
          missing=true
          printf '%s\n' "$sofar" >> "$state"
        fi
      done
    done
    sort -u -o "$state" "$state"
    if [ "${4:-}" = standin ]; then
      for path in "${!standins[@]}"; do
        if grep -qxF "$path" "$state"; then
          printf '%s\n' "${standins[$path]}" > "$tree/$path"
          printf 'standin %s\n' "$path" >> "$state"
        fi
      done
    fi
    ;;
  after)
    state="$3"
    refuse() { echo "The session left $1 at .${2#"$tree"}, where it started with nothing, so git isn't run on it."; exit 1; }
    # Before anything is removed: no recorded path, nor any folder above it in the tree, may be a
    # link, and each must resolve inside the tree, so nothing removed later acts through a link.
    root="$(realpath "$tree")"
    while IFS= read -r path; do
      sofar=""
      IFS=/ read -r -a parts <<< "$path"
      for part in "${parts[@]}"; do
        sofar="${sofar:+$sofar/}$part"
        if [ -L "$tree/$sofar" ]; then refuse "a link" "$tree/$sofar"; fi
      done
      case "$(realpath -m "$tree/$path")" in
        "$root"/*) ;;
        *) refuse "a path that leads out of the tree" "$tree/$path" ;;
      esac
    done < <(grep -v '^standin ' "$state")
    # .claude first: the sandbox's placeholders for its settings, commands, agents, hooks and skills,
    # and anything Claude Code kept there, are said in full, each name escaped, and removed, since no
    # change may carry .claude and nothing on the host reads it.
    claude=false
    if grep -qxF .claude "$state"; then
      if [ -L "$tree/.claude" ]; then refuse "a link" "$tree/.claude"; fi
      if [ -d "$tree/.claude" ]; then
        echo "The session left these in .claude, which is removed:"
        find "$tree/.claude" -mindepth 1 -printf '%P %s\n' | LC_ALL=C sort | jq -R .
        rm -rf "${tree:?}/.claude"
        claude=true
      fi
    fi
    # The rest deepest first, a path inside another sorting after it and so, reversed, before it,
    # and a folder is removed only once nothing but placeholders was in it.
    while IFS= read -r path; do
      case "$path" in .claude|.claude/*) if [ "$claude" = true ]; then continue; fi ;; esac
      at="$tree/$path"
      if [ -L "$at" ]; then refuse "a link" "$at"; fi
      if grep -qxF "standin $path" "$state"; then
        if [ ! -f "$at" ] || ! cmp -s "$at" <(printf '%s\n' "${standins[$path]}"); then refuse "a ${path##*/} other than the stand-in" "$at"; fi
        rm -f "$at"
      elif [ -f "$at" ]; then
        if [ -s "$at" ]; then refuse "a file with something in it" "$at"; fi
        rm -f "$at"
      elif [ -d "$at" ]; then
        if [ -n "$(find "$at" -mindepth 1 -print -quit)" ]; then refuse "a folder with something in it" "$at"; fi
        rmdir "$at"
      elif [ -e "$at" ]; then
        refuse "something other than a file or folder" "$at"
      fi
    done < <(grep -v '^standin ' "$state" | LC_ALL=C sort -r)
    ;;
  exclude)
    mkdir -p "$tree/.git/info"
    printf '/%s\n' "${paths[@]}" node_modules >> "$tree/.git/info/exclude"
    ;;
  *) echo "The mode must be before, after or exclude: $mode" >&2; exit 1 ;;
esac
