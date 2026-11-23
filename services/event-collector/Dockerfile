# syntax=docker/dockerfile:1
# Build context: services/  (docker build -f event-collector/Dockerfile -t examory/event-collector:dev services)
# The platform module is copied next to event-collector so the `replace ../platform`
# directive resolves inside the image.

# Latest Go 1.26 patch at the time of writing (Go 1.25 is out of support and
# has unfixed stdlib CVEs, see README). Pin by digest for releases.
FROM golang:1.26.11-alpine3.24 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY platform/go.mod platform/go.sum ./platform/
COPY event-collector/go.mod event-collector/go.sum ./event-collector/
RUN cd event-collector && go mod download
COPY platform ./platform
COPY event-collector ./event-collector
RUN cd event-collector && go build -trimpath -ldflags="-s -w" -o /out/event-collector ./cmd/event-collector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/event-collector /event-collector
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/event-collector"]
