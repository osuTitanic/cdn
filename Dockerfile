FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install ca-certificates for https requests & curl for health checks
RUN apk add --no-cache ca-certificates curl

# Using the static build from:
# https://github.com/moparisthebest/static-curl/releases
RUN curl -L -o /tmp/curl-static \
	https://github.com/moparisthebest/static-curl/releases/download/curl-amd64 \
	&& chmod +x /tmp/curl-static

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /s3-cdn .

FROM scratch

# Copy curl for health checks
COPY --from=builder /tmp/curl-static /usr/bin/curl

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /s3-cdn /s3-cdn

EXPOSE 8080
ENTRYPOINT ["/s3-cdn"]