FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o nymph .

RUN mkdir -p /tmp && chmod 1777 /tmp

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /tmp /tmp

COPY --from=builder /build/nymph /nymph

USER 65534:65534

EXPOSE 9813

ENTRYPOINT ["/nymph"]