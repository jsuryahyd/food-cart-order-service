#!/bin/bash

# scripts/generate.sh

# Exit immediately if a command exits with a non-zero status.
set -e

echo "Ensuring Go modules are updated..."
go mod tidy
# Optional: 'go mod vendor' is useful for CI/CD pipelines that prefer vendored dependencies.
# go mod vendor

echo "Generating API code from openapi.yaml..."

# Define the input OpenAPI spec file
OPENAPI_SPEC="api/openapi.yaml"

# Define the output directory for generated files
OUTPUT_DIR="api/generated"

# Create the output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Run oapi-codegen using 'go run'. This automatically handles tool installation
# if the specified version is not already in the Go module cache.

# Generate types and server interfaces
echo "  - Generating types and server interfaces (openapi_types.go)"
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
    -package generated \
    -generate types,server \
    -o "$OUTPUT_DIR/openapi_types.go" \
    "$OPENAPI_SPEC"

# Generate Gin-specific router code - Not useful
# echo "  - Generating Gin router code (openapi_gin.go)"
# go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
#     -package generated \
#     -generate gin \
#     -o "$OUTPUT_DIR/openapi_gin.go" \
#     "$OPENAPI_SPEC"

echo "API code generation complete."