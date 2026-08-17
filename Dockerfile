# Base images are pinned by digest, not just by tag — a tag can be repointed at a
# different image, a digest cannot. Renovate keeps these digests current.
FROM golang:1.26.6-alpine3.24@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS build

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

FROM gcr.io/distroless/static-debian13:nonroot@sha256:d29e660cc75a5b6b1334e03c5c81ccf9bc0884a002c6000dbf0fb96034814478

ENV DB_PATH=/data/uptime.db

COPY --from=build --chown=65532:65532 /data /data
COPY --from=build /src/uptime /uptime
COPY ./config.yml.example /config.yml

CMD ["/uptime"]
