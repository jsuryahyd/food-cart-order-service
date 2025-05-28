# Dockerfile.preprocessor

FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum for the pre-processor (assuming it's in the root module)
COPY go.mod go.sum ./
RUN go mod download

# Copy only the pre-processor source code (or the whole project if simpler)
COPY cmd/coupon_preprocessor cmd/coupon_preprocessor
COPY internal/ internal/ # If preprocessor uses internal packages
# Add other necessary files for the preprocessor, e.g., config

# Build the coupon pre-processor
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o /app/bin/coupon_preprocessor ./cmd/coupon_preprocessor/main.go

# Stage 2: Minimal image for running the pre-processor
FROM alpine:latest

WORKDIR /app

# Copy the built binary
COPY --from=builder /app/bin/coupon_preprocessor .

# Ensure the output directory exists on the volume
RUN mkdir -p /output

# The command to run this service.
# For a job, you might not want it to run on 'docker-compose up',
# but rather manually or via a cron.
# If you make it the default command, it will run and exit.
CMD ["./coupon_preprocessor"]