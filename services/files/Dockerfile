# syntax=docker/dockerfile:1
# Build context: services/  (docker build -f files/Dockerfile -t examory/files:dev services)
# The platform module is copied next to files so the `replace ../platform`
# directive resolves inside the image.

# Latest Go 1.26 patch at the time of writing (Go 1.25 is out of support and
# has unfixed stdlib CVEs, see README). Pin by digest for releases.
FROM golang:1.26.11-alpine3.24 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY platform/go.mod platform/go.sum ./platform/
COPY files/go.mod files/go.sum ./files/
RUN cd files && go mod download
COPY platform ./platform
COPY files ./files
RUN cd files && go build -trimpath -ldflags="-s -w" -o /out/files ./cmd/files

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/files /files
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/files"]
