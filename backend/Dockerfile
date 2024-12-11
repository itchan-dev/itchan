ARG PORT=8080

FROM golang:1.22.10

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY . ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /itchan-api ./backend/cmd/itchan-api

# Optional:
# To bind to a TCP port, runtime parameters must be supplied to the docker command.
# But we can document in the Dockerfile what ports
# the application is going to listen on by default.
# https://docs.docker.com/reference/dockerfile/#expose
EXPOSE 8080

# Run
CMD ["/itchan-api"]


# # Use official Golang image as the builder
# FROM golang:1.20-alpine AS builder

# # Set working directory
# WORKDIR /app

# # Copy go.mod and go.sum
# COPY go.mod go.sum ./

# # Download dependencies
# RUN go mod download

# # Copy the rest of the source code
# COPY . .

# # Build the backend binary
# RUN go build -o itchan-api ./cmd/itchan-api/

# # Use a minimal image for the final build
# FROM alpine:latest

# # Set working directory
# WORKDIR /root/

# # Copy the binary from the builder
# COPY --from=builder /app/itchan-api .

# # Expose port (optional, since Docker Compose handles it)
# EXPOSE 8000

# # Command to run the backend
# CMD ["./itchan-api"]
