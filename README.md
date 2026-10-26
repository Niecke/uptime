# uptime
Standalone uptime tracker with a small dashboard for all endpoints configured.

## Setup Go

```bash
# download and afterwards export to /usr/local
tar -C /usr/local -xzf go1.26.1.linux-amd64.tar.gz

# add to ~/.bash_profile
export PATH=$PATH:/usr/local/go/bin

# reload bash profile
source ~/.bash_profile
```

## Docker/Podman

```
podman build -t uptime .

# with default config
podman run -it -v ./data:/data:Z -p 3333:3333 uptime:latest

# with custom config
podman run -it -v ./data:/data:Z -v ./config.yml:/config.yml:z -p 3333:3333 uptime:latest
```

## Local Dev

When running with DEV=true, the HTML content is reloaded from disk.
```bash
DEV=true go run cmd/main.go -config=./config.yml
```

## Branches and releases

Development is trunk-based: branches start from `main` and come back to `main`
through a pull request. Pull requests are squash-merged, and the pull request
title is the commit message, so it has to be a
[Conventional Commit](https://www.conventionalcommits.org/) such as
`fix: record the status of a timed-out check` or `feat: show the response time`.
The **commitlint** workflow checks the title.

[release-please](https://github.com/googleapis/release-please) reads those
commits and keeps one release pull request, `chore(main): release X.Y.Z`, open
against `main`. Merging it is the release decision. Nobody edits a version or
creates a tag by hand: the version lives in `.release-please-manifest.json` and
`CHANGELOG.md` is written by release-please.

The image is built exactly once, on the pull request, after the suite passes.
Every step after that retags that same digest, so the bytes that were tested are
the bytes that get deployed.

| Event | Images | Git |
| --- | --- | --- |
| Pull request | tests, then build and push Artifact Registry `ci-<head sha>` | |
| Push to `main` | retag as Artifact Registry `sha-<short sha>` (`promote.yml`) | release pull request opened or updated (`release-please.yml`) |
| Release pull request merged | retag as `X.Y.Z`, `X.Y`, `X`, `latest` on Artifact Registry and Docker Hub (`release.yml`) | tag `vX.Y.Z`, GitHub Release with the changelog and the image digest |

Nothing is retested or rebuilt on a merge. A pull request is tested as the merge
preview of head and base, which is the same tree the merge commit produces, and
retagging cannot change what is inside the image.

The version compiled into the binary is `dev-<short sha>` on every ordinary
pull request. On the release pull request it is the version in
`.release-please-manifest.json`, so the digest that gets promoted to `X.Y.Z`
reports `X.Y.Z`. The git hash, also compiled in, is what tells two builds of the
same version apart. Both are on `/version` and on every log line.

### What starts a run

Tests, images and releases only happen when the change can reach the binary or
the image: `cmd/`, `internal/`, `go.mod`, `go.sum`, `Dockerfile`,
`.dockerignore`, `config.yml.example`, `.release-please-manifest.json`, and
`.github/` so that a pipeline change is exercised by the pipeline. A pull
request that only touches the README, `compose.yml` or the `Caddyfile` reports
its checks as skipped and merges without building anything.

The filter is a condition on the jobs, not a `paths:` filter on the triggers. A
workflow skipped by path filtering never reports its checks at all, which would
leave a required check pending forever and block the merge.

### Cutting a release

Merge the release pull request that release-please keeps open. That is all: it
tags `vX.Y.Z` and publishes the GitHub Release, the tag starts `release.yml`, and
that retags the image `promote.yml` tagged `sha-<short sha>` for the merge. The
release job waits up to ten minutes for that tag, because both workflows start
on the same push.

Because releases promote rather than build, a commit that reaches `main` without
a pull request has no tested image behind it, and the release fails saying so
rather than shipping something nothing ever checked. A release pull request
that changes no code is the same case: the `Build image` check reports `skipped`
and there is nothing to promote.

`compose.yml` is the record of what runs on the server. The release does not
write it; Renovate raises a pull request for the pinned image like any other
dependency, and deploying is still manual.

The release pull request must be opened with a token that can start workflows,
otherwise its checks never run and a required check cannot be satisfied. Add a
fine-grained personal access token with `Contents` and `Pull requests`
read/write as the repository secret `RELEASE_PLEASE_TOKEN`. Without it
release-please falls back to `GITHUB_TOKEN`: the release pull request is still
opened, but it gets no CI run and the tag it pushes does not start `release.yml`.

### Branch protection

`main` should require a pull request, require the `commitlint`, `tests` and
`Build image` checks, and block force pushes and deletions. Requiring the build
matters because releases promote rather than rebuild: a merge with no image
behind it cannot be released. `tests` and `Build image` report `skipped` on a
pull request that changes no code, which satisfies them.

Add the ruleset in Settings, Rules, Rulesets, or from the command line:

```bash
gh api --method POST repos/Niecke/uptime/rulesets --input - <<'JSON'
{
  "name": "main",
  "target": "branch",
  "enforcement": "active",
  "conditions": { "ref_name": { "include": ["refs/heads/main"], "exclude": [] } },
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    { "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 0,
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false
      } },
    { "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": false,
        "required_status_checks": [
          { "context": "commitlint" },
          { "context": "tests" },
          { "context": "Build image" }
        ]
      } }
  ]
}
JSON
```
