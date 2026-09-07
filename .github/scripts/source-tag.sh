#!/usr/bin/env bash
#
# Names the image CI built for the change that is now on this branch.
#
# Merging creates a new commit, so a branch cannot name its own image: images
# are tagged with the sha of the pull request head that was tested. This walks
# back from the merge commit to that head.
#
# Prints a tag like `ci-1a2b3c4`. Never fails: a commit with no pull request
# behind it falls back to itself, and the caller finds no such image and says so.
set -euo pipefail

sha="${1:-$GITHUB_SHA}"

# Anything that is not a full sha is treated as nothing. A failing `gh api`
# writes its error body to stdout, so without this a 404 would end up spliced
# into the image tag.
sha_or_empty() {
  local value="$1"
  if [[ $value =~ ^[0-9a-f]{40}$ ]]; then
    printf '%s' "$value"
  fi
}

# The API knows which pull request a merge commit came from, including squash
# and rebase merges, where the commit has no second parent to follow.
head="$(sha_or_empty "$(gh api "repos/$GITHUB_REPOSITORY/commits/$sha/pulls" --jq '.[0].head.sha // empty' 2>/dev/null || true)")"

if [[ -z $head ]]; then
  # A merge commit keeps the pull request head as its second parent, which
  # covers a merge pushed without the API having caught up.
  head="$(sha_or_empty "$(git rev-parse --verify --quiet "$sha^2" || true)")"
fi

if [[ -z $head ]]; then
  head="$sha"
fi

printf 'ci-%s\n' "${head:0:7}"
