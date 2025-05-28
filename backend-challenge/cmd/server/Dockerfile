# Dockerfile.app

# Stage 1: Build the Go application
FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the main API server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o /app/bin/server ./cmd/server/main.go

# Stage 2: Create a minimal production image for the app
FROM alpine:latest

WORKDIR /app

# Copy the built API server binary
COPY --from=builder /app/bin/server .

# Create the data directory where the volume will be mounted
RUN mkdir -p /app/data

# Copy static assets (like openapi.yaml) ???
COPY api/openapi.yaml api/openapi.yaml
COPY configs/ configs/
COPY web/ web/

# Expose the port your Go application listens on
EXPOSE 8080

# Command to run the executable
CMD ["/app/server"]