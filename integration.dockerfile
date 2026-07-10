FROM golang:1.21.12 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY Makefile Makefile

# Copy the go source
COPY apis/ apis/
COPY config/ config
COPY controllers/ controllers/
COPY events/ events/
COPY hack/ hack/
COPY metrics/ metrics/
COPY service/ service/
COPY util/ util/

# Download dependencies
RUN make envtest 
RUN make controller-gen

# CMD ["make", "test-local"]
CMD ["make", "int-test"]