# syntax=docker/dockerfile:1
# Build context: services/  (docker build -f assignments/Dockerfile -t examory/assignments:dev services)
# The platform module is copied next to assignments so the `replace ../platform`
# directive resolves inside the image.

# Latest Go 1.26 patch at the time of writing (Go 1.25 is out of support and
# has unfixed stdlib CVEs, see README). Pin by digest for releases.
FROM golang:1.26.11-alpine3.24 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY platform/go.mod platform/go.sum ./platform/
COPY assignments/go.mod assignments/go.sum ./assignments/
RUN cd assignments && go mod download
COPY platform ./platform
COPY assignments ./assignments
RUN cd assignments && go build -trimpath -ldflags="-s -w" -o /out/assignments ./cmd/assignments

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/assignments /assignments
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/assignments"]
