#### FIRST STAGE ####
FROM golang:latest as builder

# Set the directory
WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source
COPY main.go /app/
COPY static /app/static

# Build
# NOTE: CGO_ENABLED=0 is required to build a binary that works in an alpine container
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o snorlax .


#### SECOND STAGE ####
FROM alpine:latest

# Link the GitHub package to this repo
LABEL org.opencontainers.image.source https://github.com/moonbeam-nyc/snorlax

# Set the directory
WORKDIR /app

# Copy the binary from the previous stage
COPY --from=builder /app/snorlax .

# Set the default command
CMD ["./snorlax", "serve"]