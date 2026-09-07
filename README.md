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

Work lands on `dev` and is released by merging `dev` into `main`. The `VERSION`
file at the repo root is the only place a version is written by hand: it holds
the release the current cycle is working towards.

The image is built exactly once, on the pull request, after the suite passes.
Every step after that retags that same digest, so the bytes that were tested are
the bytes that get deployed.

| Event | Images | Git |
| --- | --- | --- |
| Pull request | tests, then build and push Artifact Registry `ci-<head sha>` | |
| Push to `dev` | retag as Artifact Registry `0.2.0-dev.N` and `dev` | |
| Push to `main` | retag as `0.2.0`, `0.2`, `0`, `latest` on Artifact Registry, copied to Docker Hub | tag `v0.2.0` and release notes |

`N` counts the commits since the last release tag, so it only grows within a
cycle and restarts at the next release.

Nothing is retested or rebuilt on a merge. A pull request is tested as the merge
preview of head and base, which is the same tree the merge commit produces, and
retagging cannot change what is inside the image.

The version compiled into the binary is what `VERSION` says, `0.2.0`, without
the `-dev.N` suffix. That is what lets one build serve the whole cycle: the
digest promoted to `0.2.0` reports 0.2.0 because it was always going to be
0.2.0. The pre-release counter lives in the image tag, and the git hash, also
compiled in, is what tells two builds of the same version apart. Both are on
`/version` and on every log line.

### What starts a run

Tests, images and releases only happen when the change can reach the binary or
the image: `cmd/`, `internal/`, `go.mod`, `go.sum`, `Dockerfile`,
`.dockerignore`, `config.yml.example`, `VERSION`, and `.github/` so that a
pipeline change is exercised by the pipeline. A pull request that only touches
the README, `compose.yml` or the `Caddyfile` reports its checks as skipped and
merges without building anything.

The filter is a condition on the jobs, not a `paths:` filter on the triggers. A
workflow skipped by path filtering never reports its checks at all, which would
leave a required check pending forever and block the merge.

### Cutting a release

1. Run the **Open release PR** workflow from the Actions tab. Leave `bump` on
   `none` to ship the version dev has been building, or pick `patch`, `minor` or
   `major` to decide at cut time. It rewrites `VERSION` on dev if needed and
   puts a link in the run summary that opens the pull request with the title and
   changelog already filled in.
2. Follow the link, create the pull request, merge it.

The rest is automatic: the image that pull request built and tested is retagged
with the release version on both registries, `v0.2.0` is tagged with generated
release notes, the image line in `compose.yml` is repinned to the released tag
and digest, and `VERSION` on dev moves to the next patch so the following cycle
can start.

`compose.yml` is the record of what runs on the server, so the release writes it
rather than Renovate: at that moment the tag has just been published and the
digest has just been checked identical on both registries. Renovate is turned
off for this repository's own image and still watches Caddy. Both writes land on
dev, so the copy on `main` is one release behind until the next merge, the same
as `VERSION`.

Deploying is still manual. The pin says what to run, not that it is running.

Because releases promote rather than build, a commit that reaches `main` without
a pull request has no tested image behind it, and the release fails saying so
rather than shipping something nothing ever checked.

A push to `main` that does not change `VERSION` finds its tag already published
and stops before building, so documentation fixes and reverts on main are safe.

### Branch protection

`main` should require a pull request, require the `tests` and `Build image`
checks, and block force pushes and deletions. Requiring the build matters now
that releases promote rather than rebuild: a merge with no image behind it
cannot be released. Both checks report `skipped` on a pull request that changes
no code, which satisfies them. The release pull request is opened by a person
from the link the workflow produces, precisely so that its checks run and a
required check is satisfiable.

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
          { "context": "tests" },
          { "context": "Build image" }
        ]
      } }
  ]
}
JSON
```

`dev` stays unprotected. The release workflow writes the next `VERSION` to it
directly, and a rule requiring pull requests there would block that.
