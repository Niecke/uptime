#!/usr/bin/env bash
#
# The one place a version is worked out. Every workflow calls this instead of
# parsing tags on its own.
#
# VERSION holds the release the current dev cycle is working towards. From that
# file plus the git history this derives:
#
#   version      0.2.0        what a release cut from main will be tagged
#   minor        0.2          rolling tags for the container registries
#   major        0
#   dev_version  0.2.0-dev.7  same version, plus commits since the last release
#   latest       v0.1.4       newest release tag, empty before the first release
#   released     true|false   whether v<version> is already tagged
#
# Values are echoed and, under Actions, appended to $GITHUB_OUTPUT.
#
# It fails when VERSION is not strict semver, or has fallen behind the newest
# release tag. Left alone that mistake ends with two different builds sharing a
# Docker tag, which is the one failure mode a digest pin downstream cannot see.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

version="$(tr -d '[:space:]' < VERSION)"

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error file=VERSION::VERSION must be MAJOR.MINOR.PATCH, found '$version'" >&2
  exit 1
fi

# Release tags only. Dev builds are not tagged in git, the repo carries a stray
# non-version tag, and a future -rc tag would sort in here too, so match the
# shape exactly rather than trusting "the newest tag".
latest="$(git tag --list --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"

if [[ -n $latest ]]; then
  oldest="$(printf '%s\n%s\n' "$version" "${latest#v}" | sort -V | head -n1)"
  if [[ $oldest == "$version" && $version != "${latest#v}" ]]; then
    echo "::error file=VERSION::VERSION ($version) is behind the last release ($latest). Raise it." >&2
    exit 1
  fi
fi

# Commits since the last release number the dev builds. The count only grows
# inside a cycle and returns to zero once the next release is tagged.
if [[ -n $latest ]]; then
  count="$(git rev-list --count "$latest..HEAD")"
else
  count="$(git rev-list --count HEAD)"
fi

if git rev-parse -q --verify "refs/tags/v$version" >/dev/null; then
  released=true
else
  released=false
fi

emit() {
  printf '%s=%s\n' "$1" "$2"
  if [[ -n ${GITHUB_OUTPUT:-} ]]; then
    printf '%s=%s\n' "$1" "$2" >> "$GITHUB_OUTPUT"
  fi
}

emit version     "$version"
emit minor       "${version%.*}"
emit major       "${version%%.*}"
emit dev_version "$version-dev.$count"
emit latest      "$latest"
emit released    "$released"
