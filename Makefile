.PHONY: start test show_coverage

run:
	docker-compose up

build_run:
	docker-compose up --build

test:
	go test ./...

show_coverage:
	go test ./... -coverprofile fmtcoverage.html
	go tool cover -html fmtcoverage.html
