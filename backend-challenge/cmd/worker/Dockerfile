# Stage 1: Build the Go application
FROM golang:1.23-bookworm AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files.
# This assumes your go.mod and go.sum are in the project root.
# If they are in cmd/worker, adjust the paths accordingly.
COPY go.mod go.sum ./

# Download all dependencies. This step leverages Docker's layer caching.
# If go.mod and go.sum don't change, this step won't re-run.
RUN go mod download

# Copy the entire project source code.
# This is generally safer for complex projects where multiple packages
# might be used, including those in 'internal'.
# If you only want to copy 'cmd/worker' and its direct dependencies,
# you'd need to be more precise.
COPY . .

# Build the coupon pre-processor.
# CGO_ENABLED=0 creates a statically linked binary, making the final image smaller.
# GOOS=linux ensures it's built for Linux (the Alpine base image).
# -a -installsuffix cgo are often used together to ensure a clean build,
# but can sometimes be omitted with CGO_ENABLED=0.
# -ldflags="-s -w" removes symbol tables and debug information, further reducing binary size.
# The output binary is named 'coupon_preprocessor' and placed in /app/bin/.
# The path './cmd/worker/main.go' correctly points to your entry file.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o /app/bin/coupon_preprocessor ./cmd/worker/main.go

# Stage 2: Create a minimal image to run the pre-processor
FROM alpine:latest

# Set the working directory for the final image
WORKDIR /app

# Copy the built executable from the builder stage into the final image
COPY --from=builder /app/bin/coupon_preprocessor .
COPY configs/ configs/
# Create the /output directory inside the container.
# This is crucial because the 'processed_coupons_data' volume is mounted
# to '/output' within the coupon_preprocessor container, and the Go application
# expects this directory to exist when it tries to write to OUTPUT_FILE_PATH.
RUN mkdir -p /output
EXPOSE 6060
# The command to run the executable.
# As discussed, this service is designed to run and exit after processing,
# or to continuously poll Redis in a more advanced setup.
CMD ["./coupon_preprocessor"]