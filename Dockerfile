# Base images are pinned by digest, not just by tag — a tag can be repointed at a
# different image, a digest cannot. Renovate keeps these digests current.
FROM golang:1.27.3-alpine3.24@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build

ARG BUILD_GIT_HASH="unknown"
ARG BUILD_VERSION="dev"

RUN mkdir /src
WORKDIR /src

# Copy dependency files first — this layer is cached until go.mod/go.sum change
COPY go.mod go.sum ./
# go mod verify re-checks every downloaded module against the hashes in go.sum
RUN go mod download && go mod verify

COPY ./ /src

# -mod=readonly fails the build if go.mod/go.sum would have to change, so a dependency
# can never be silently added or upgraded during an image build.
# -trimpath keeps local filesystem paths out of the binary.
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -o uptime \
    -ldflags="-w -s \
      -X 'niecke-it.de/uptime/internal/version.GitHash=${BUILD_GIT_HASH}' \
      -X 'niecke-it.de/uptime/internal/version.Version=${BUILD_VERSION}'" \
    cmd/main.go

# distroless has no shell, so create /data here and copy it into the final image
RUN mkdir /data && chown -R 65532:65532 /data

FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931

ENV DB_PATH=/data/uptime.db

COPY --from=build --chown=65532:65532 /data /data
COPY --from=build /src/uptime /uptime
COPY ./config.yml.example /config.yml

CMD ["/uptime"]
