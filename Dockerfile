# The base image is already multi-architecture, supporting both amd64 and arm64.
FROM golang:1.22-alpine as builder

WORKDIR /workspace
COPY . .

# The CGO_ENABLED=0 environment variable ensures a statically linked binary is produced,
# which is ideal for compatibility across different Linux distributions and architectures.
# Removed GOARCH specification here to allow build parameter control architecture.
RUN CGO_ENABLED=0 GOOS=linux go build -a -o dd-csi-driver cmd/dd-csi-driver/main.go

# Using a specific version increases reproducibility.
# Alpine's latest tag supports multiple architectures.
FROM alpine:latest

# It's generally a good practice to include ca-certificates.
RUN apk add --no-cache ca-certificates

COPY --from=builder /workspace/dd-csi-driver /dd-csi-driver

ENTRYPOINT ["/dd-csi-driver"]