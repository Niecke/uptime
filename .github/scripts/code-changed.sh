#!/usr/bin/env bash
#
# Decides whether a range of commits touches anything that can change the
# binary or the image. Prints `true` or `false`.
#
#   code-changed.sh <base> <head> [pull_request]
#
# A third argument of `pull_request` compares against the merge base, so commits
# that landed on the target branch while the pull request was open are not
# mistaken for its own changes.
#
# Anything outside the list below — the README, the deployment compose file, the
# Caddyfile, the live config — cannot alter what is built, so it runs no tests
# and produces no image. It errs towards `true`: an unknown base, a force push,
# a brand new branch, anything it cannot reason about runs the full pipeline.
set -euo pipefail

base="${1:-}"
head="${2:-}"
event="${3:-push}"

# Everything the build reads. VERSION is here because it decides the tags a
# release publishes, and .github/ because a change to the pipeline should be
# exercised by the pipeline.
pattern='^(cmd/|internal/|go\.mod$|go\.sum$|Dockerfile$|\.dockerignore$|config\.yml\.example$|VERSION$|\.github/(workflows|scripts)/)'

# The all-zero sha is what a push event carries for a branch that did not exist
# before, and there is nothing to diff against.
if [[ -z $base || $base =~ ^0+$ || -z $head ]]; then
  echo "No usable base ('$base') — treating this as a code change." >&2
  echo true
  exit 0
fi

if ! git cat-file -e "$base^{commit}" 2>/dev/null; then
  echo "Base $base is not in this clone — treating this as a code change." >&2
  echo true
  exit 0
fi

if [[ $event == pull_request ]]; then
  range="$base...$head"
else
  range="$base..$head"
fi

# Captured rather than piped into grep: grep exits at the first match, and the
# SIGPIPE that gives git would fail the whole pipeline under `set -o pipefail`.
changed="$(git diff --name-only "$range")"
echo "$changed" >&2

if grep -qE "$pattern" <<< "$changed"; then
  echo true
else
  echo false
fi
