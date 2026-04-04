FROM golang:1.26.1-alpine3.22 AS build

RUN mkdir /src
WORKDIR /src

COPY ./ /src

RUN CGO_ENABLED=0 go build -o uptime -ldflags="-w -s" cmd/main.go

FROM gcr.io/distroless/static-debian13

ENV DB_PATH=/data/uptime.db

COPY --from=build /src/uptime /uptime

CMD ["/uptime"]
