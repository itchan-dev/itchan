.PHONY: start show_coverage

run:
	docker-compose up

show_coverage:
	go test ./... -coverprofile fmtcoverage.html
	go tool cover -html fmtcoverage.html
