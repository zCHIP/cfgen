### STAGE 1: build executable binaries
FROM golang:alpine as builder

# Install GIT and SSL CA certificates
RUN echo "Installing dependencies" \
    && apk update \
    && apk add --no-cache git ca-certificates \
    && update-ca-certificates

# Create appuser
RUN adduser -D -g '' appuser

# Copy the sources to build
COPY . /go/src/cfgen/
WORKDIR /go/src/cfgen/

# Enable GO modules and fetch the dependencies
ENV GO111MODULE=on
RUN go mod download

# Build CLI binary
RUN cd cfgencli \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/cfgencli

# Build cfgensvc binary
RUN cd cfgensvc \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/cfgensvc

### STAGE 2: build the image
FROM scratch

# Import from builder certs and passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd

# Copy the static executables
COPY --from=builder ["/go/bin/cfgencli", "/go/bin/cfgensvc", "/go/bin/"]

# Use an unprivileged user
USER appuser

# Run the service binary by default
ENTRYPOINT ["/go/bin/cfgensvc"]
