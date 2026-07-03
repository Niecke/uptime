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

## Release

A new release is created by adding a tag to the main branch. 
```bash
git tag v0.1.0
git push origin v0.1.0
```
