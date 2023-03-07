# Build the manager binary
FROM golang:1.18 as builder
MAINTAINER "Avesha Systems"
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY service/ service/
COPY util/ util/
COPY cleanup/ cleanup/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o cleanup ./cleanup/

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/cleanup/cleanup .
# Copy config files for event monitoring
COPY files files
ENV EVENT_SCHEMA_PATH="/files/events"
USER 65532:65532

ENTRYPOINT ["/manager"]
