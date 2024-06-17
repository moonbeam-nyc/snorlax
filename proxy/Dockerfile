# Start from a base image with Go installed
FROM golang:latest as builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o snorlax-proxy .


# Use a minimal base image
FROM alpine:latest

# Link the GitHub package to this repo
LABEL org.opencontainers.image.source https://github.com/moonbeam-nyc/snorlax

# Set the working directory inside the container
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/snorlax-proxy .

# Run the binary
CMD ["./snorlax-proxy"]