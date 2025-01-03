.PHONY: start test show_coverage

run:
	docker-compose up

test:
	go test ./...

show_coverage:
	go test ./... -coverprofile fmtcoverage.html
	go tool cover -html fmtcoverage.html
