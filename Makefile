.PHONY: up build-up test show-coverage

up:
	docker-compose up

build-up:
	docker-compose up --build

test:
	go test ./...

show-coverage:
	go test ./... -coverprofile fmtcoverage.html
	go tool cover -html fmtcoverage.html
