FROM esacteksab/go:1.26.2-2026-04-10@sha256:90b268a55338c4b88dfd7ead1b456d6996d37e2703e7c94778340b6096251960 AS builder

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
