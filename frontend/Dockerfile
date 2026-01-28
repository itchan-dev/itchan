FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o frontend ./frontend/cmd/frontend/

FROM alpine:latest

# Create non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/frontend/frontend .
COPY --from=builder /app/config ./config/
COPY --from=builder /app/frontend/templates ./templates/
COPY --from=builder /app/frontend/static ./static/

# Create media directory
RUN mkdir -p ./media

# Set ownership for non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

EXPOSE 8081
CMD ["./frontend", "-config_folder", "./config"]
