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

| Event | Images | Git |
| --- | --- | --- |
| Pull request into `dev` | none, tests only | |
| Push to `dev` | Artifact Registry `0.2.0-dev.N`, `dev`, `<sha>` | |
| Push to `main` | Artifact Registry and Docker Hub `0.2.0`, `0.2`, `0`, `latest` | tag `v0.2.0` and release notes |

`N` counts the commits since the last release tag, so it only grows within a
cycle and restarts at the next release. The version is compiled into the binary
and is what `/version` and every log line report.

### Cutting a release

1. Run the **Open release PR** workflow from the Actions tab. Leave `bump` on
   `none` to ship the version dev has been building, or pick `patch`, `minor` or
   `major` to decide at cut time. It rewrites `VERSION` on dev if needed and
   puts a link in the run summary that opens the pull request with the title and
   changelog already filled in.
2. Follow the link, create the pull request, merge it.

The rest is automatic: the suite runs on the merge commit, the images are built
with the release version compiled in and pushed to both registries, `v0.2.0` is
tagged with generated release notes, and `VERSION` on dev moves to the next
patch so the following cycle can start. Renovate then raises the usual pull
request to bump the pinned image in `compose.yml`.

A push to `main` that does not change `VERSION` finds its tag already published
and stops before building, so documentation fixes and reverts on main are safe.

### Branch protection

`main` should require a pull request, require the `tests` check, and block force
pushes and deletions. The release pull request is opened by a person from the
link the workflow produces, precisely so that its checks run and a required
check is satisfiable.

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
        "required_status_checks": [ { "context": "tests" } ]
      } }
  ]
}
JSON
```

`dev` stays unprotected. The release workflow writes the next `VERSION` to it
directly, and a rule requiring pull requests there would block that.
