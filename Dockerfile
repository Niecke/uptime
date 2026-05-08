FROM golang:1.26.3-alpine3.22 AS build

RUN mkdir /src
WORKDIR /src

# Copy dependency files first — this layer is cached until go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

COPY ./ /src
RUN CGO_ENABLED=0 go build -o uptime -ldflags="-w -s" cmd/main.go

FROM gcr.io/distroless/static-debian13

ENV DB_PATH=/data/uptime.db

COPY --from=build /src/uptime /uptime
COPY ./config.yml.example /config.yml

CMD ["/uptime"]
