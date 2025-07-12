FROM esacteksab/go:1.24.5-2025-07-11@sha256:a84a4194b6ee681dddbe96dd6642b719c3f2c9f5e0d2bb40d6553a5668a72374 AS builder

# Set GOMODCACHE explicitly (still good practice)
ENV GOMODCACHE=/go/pkg/mod

WORKDIR /app

# Copy only module files first to maximize caching
COPY go.mod go.sum ./

# Download modules. This layer will be cached if go.mod/go.sum haven't changed.
# The downloaded files will now be part of this layer's filesystem.
RUN go mod download

# Copy the rest of the application code
COPY . .

# Keep cache mounts here for build performance (Go build cache + reusing modules during build)
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build scripts/build-dev.sh
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build scripts/help-docker.sh

# --- Test Stage ---
FROM builder AS test-stage

# No need to set GOMODCACHE again, inherited from builder
# No need to set WORKDIR again, inherited from builder

RUN mkdir -p /app/coverdata
ENV GOCOVERDIR=/app/coverdata

# Go test should now find modules in /go/pkg/mod inherited from the builder stage
CMD ["/bin/sh", "-c", "go test -covermode=atomic -coverprofile=/app/coverdata/coverage.out ./... && echo 'Coverage data collected'"]
