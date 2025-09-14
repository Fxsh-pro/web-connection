# syntax=docker/dockerfile:1.7

FROM golang:1.22-alpine AS build
WORKDIR /app
RUN apk add --no-cache build-base git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/webconnector ./cmd/webconnector

FROM alpine:3.20
WORKDIR /srv
RUN adduser -D -H app && chown -R app:app /srv
USER app
COPY --from=build /out/webconnector /usr/local/bin/webconnector
COPY web ./web
ENV LISTEN_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/webconnector"]

