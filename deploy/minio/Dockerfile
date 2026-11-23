# The upstream prebuilt images for these release tags may be unavailable.
# The Go module proxy verifies the exact corresponding Git revisions with SumDB.
FROM golang:1.26.11-alpine3.24 AS builder
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
ARG MODULE=minio
ARG VERSION=v0.0.0-20241013133411-d10bb7e1b667
RUN go mod download github.com/minio/${MODULE}@${VERSION} && \
    cd "$(go env GOMODCACHE)/github.com/minio/${MODULE}@${VERSION}" && \
    go build -trimpath -o /out/app .
FROM alpine:3.24
COPY --from=builder /out/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/app"]
