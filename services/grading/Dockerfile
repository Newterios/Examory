# syntax=docker/dockerfile:1
# Build context: services/  (docker build -f grading/Dockerfile -t examory/grading:dev services)
# The platform module is copied next to grading so the `replace ../platform`
# directive resolves inside the image.

# Latest Go 1.26 patch at the time of writing (Go 1.25 is out of support and
# has unfixed stdlib CVEs, see README). Pin by digest for releases.
FROM golang:1.26.11-alpine3.24 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY platform/go.mod platform/go.sum ./platform/
COPY grading/go.mod grading/go.sum ./grading/
RUN cd grading && go mod download
COPY platform ./platform
COPY grading ./grading
RUN cd grading && go build -trimpath -ldflags="-s -w" -o /out/grading ./cmd/grading

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/grading /grading
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/grading"]
