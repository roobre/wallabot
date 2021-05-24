FROM golang:alpine as builder

# Badger can use zstd compression via cgo
RUN apk add gcc libc-dev

WORKDIR /src

# Cache go modules layer independently from the rest of the source files
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . ./
RUN go build -o wallabot ./cmd

FROM alpine:latest
RUN apk add --no-cache tini
COPY --from=builder /src/wallabot /usr/local/bin
ENTRYPOINT [ "/sbin/tini", "--",  "/usr/local/bin/wallabot" ]
