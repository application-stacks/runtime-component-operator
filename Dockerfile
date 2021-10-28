# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY common/ common/
COPY utils/ utils/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod vendor -a -o manager main.go


#Build final image
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

LABEL vendor="Runtime Component Community" \
      name="Runtime Component Operator" \
      version="0.7.1" \
      summary="Image for Runtime Component Operator" \
      description="This image contains the controller for Runtime Component Operator. See https://github.com/application-stacks/runtime-component-operator"
      
COPY LICENSE /licenses/

WORKDIR /
COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
