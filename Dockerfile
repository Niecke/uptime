FROM golang:1.26.4-alpine3.22 AS build

ARG BUILD_GIT_HASH="unknown"

RUN mkdir /src
WORKDIR /src

# Copy dependency files first — this layer is cached until go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

COPY ./ /src
RUN CGO_ENABLED=0 go build -o uptime \
    -ldflags="-w -s -X 'niecke-it.de/uptime/internal/version.GitHash=${BUILD_GIT_HASH}'" \
    cmd/main.go

# distroless has no shell, so create /data here and copy it into the final image
RUN mkdir /data

FROM gcr.io/distroless/static-debian13:nonroot

ENV DB_PATH=/data/uptime.db

COPY --from=build /data /data
COPY --from=build /src/uptime /uptime
COPY ./config.yml.example /config.yml

CMD ["/uptime"]
