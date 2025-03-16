FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o frontend ./frontend/cmd/frontend/

FROM alpine:latest
ENV JWT_SECRET=0e93ba7e182bc111d84474103037684c7af855a5fa33499eb58df5d31021914cf4df3051ab14fdb7183fef3d1b0bf5717e0da727e5c464459d4aba7b5dc637c11d669ba819dda2da8825a4b8eca63c9dc6eedece5ed5f9ee14620a9d134d53163f4110bbb91bacbd3aef2d544ec18db53511359da7efd4801f6d9508475861469be1f91485ae7ff28767ddebce2c0a31d75857bbb8f2173c092e4cad3983557b5064bbdea590fc128a19d4bc02ee8864a74b271163ca35e439f574f4945ddbf0d61156e43030cf94dcfaf85079f8f767bc21508e07dab05854a73ccb0c25056cee55460d55b587c57d648ac7d93924f0597b26ef431318850cf57d0746b48b04
ENV ENV=development
WORKDIR /root/
COPY --from=builder /app/frontend/frontend .
# COPY --from=builder /app/frontend/templates ./templates
# COPY --from=builder /app/frontend/static ./static
EXPOSE 8081
CMD ["./frontend"]
