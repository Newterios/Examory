# syntax=docker/dockerfile:1
# Build context: services/  (docker build -f courses/Dockerfile -t examory/courses:dev services)
# The platform module is copied next to courses so the `replace ../platform`
# directive resolves inside the image.

# Latest Go 1.26 patch at the time of writing (Go 1.25 is out of support and
# has unfixed stdlib CVEs, see README). Pin by digest for releases.
FROM golang:1.26.11-alpine3.24 AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
WORKDIR /src
COPY platform/go.mod platform/go.sum ./platform/
COPY courses/go.mod courses/go.sum ./courses/
RUN cd courses && go mod download
COPY platform ./platform
COPY courses ./courses
RUN cd courses && go build -trimpath -ldflags="-s -w" -o /out/courses ./cmd/courses

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/courses /courses
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/courses"]
