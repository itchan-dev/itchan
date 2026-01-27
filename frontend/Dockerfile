FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o frontend ./frontend/cmd/frontend/

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/frontend/frontend .
COPY --from=builder /app/config ./config/
# COPY --from=builder /app/frontend/templates ./templates
# COPY --from=builder /app/frontend/static ./static
EXPOSE 8081
CMD ["./frontend", "-config_folder", "./config"]
