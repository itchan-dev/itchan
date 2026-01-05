# Use official Golang image as the builder
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the backend binary
RUN go build -o itchan-api ./backend/cmd/itchan-api/

# Use a minimal image for the final build
FROM alpine:latest

# Install ffmpeg for video metadata stripping
RUN apk add --no-cache ffmpeg

# Set working directory
WORKDIR /root/

# Copy the binary from the builder
COPY --from=builder /app/itchan-api .
COPY --from=builder /app/config ./config/

# Expose port (optional, since Docker Compose handles it)
EXPOSE 8000

# Command to run the backend
CMD ["./itchan-api", "-config_folder", "./config"]
