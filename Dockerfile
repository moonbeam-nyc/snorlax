#### FIRST STAGE ####
FROM golang:latest

# Set the directory
WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source
COPY main.go /app/
COPY static /app/static

# Build
RUN CGO_ENABLED=0 go build -o snorlax .


#### SECOND STAGE ####
FROM alpine:latest

# Set the directory
WORKDIR /app

# Copy the binary from the previous stage
COPY --from=0 /app/snorlax .

# Set the default command
CMD ["./snorlax", "watch-serve"]