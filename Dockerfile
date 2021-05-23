FROM golang:alpine as builder

WORKDIR /src
COPY . /src
RUN CGO_ENABLED=0 go build -o wallabot ./cmd

FROM alpine:latest
RUN apk add --no-cache tini
COPY --from=builder /src/wallabot /usr/local/bin
ENTRYPOINT [ "/sbin/tini", "--",  "/usr/local/bin/wallabot" ]
