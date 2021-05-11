# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /workspace

# Install Kubebuilder
ARG OS_ARCH=amd64
ARG KUBEBUILDER_VERSION=2.3.1
RUN curl -L -O "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_${OS_ARCH}.tar.gz"
RUN tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_${OS_ARCH}.tar.gz
RUN mv kubebuilder_${KUBEBUILDER_VERSION}_linux_${OS_ARCH} kubebuilder && mv kubebuilder /usr/local/
RUN export PATH=$PATH:/usr/local/kubebuilder/bin

COPY . .

# Build
RUN go mod download
RUN go fmt ./...
RUN go vet ./...
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
