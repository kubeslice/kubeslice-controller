# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
LABEL maintainer="Avesha Systems"
ARG TARGETOS
ARG TARGETARCH
ARG BUILDPLATFORM
WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.sum ./
RUN go mod download


# Copy the go source
COPY . .
# Cross-compile with optimizations and caching
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    go build -ldflags="-w -s" -trimpath -o manager main.go && \
    go build -ldflags="-w -s" -trimpath -o cleanup ./cleanup/

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/cleanup .
USER 65532:65532
ENTRYPOINT ["/manager"]
