FROM golang:1.22.10

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /itchan-frontend ./frontend

EXPOSE 8081

CMD ["/itchan-frontend"]