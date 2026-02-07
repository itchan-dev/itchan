.PHONY: up down dev test show-coverage deploy deploy-monitoring logs logs-frontend logs-api

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

build:
	docker compose -f docker-compose.yml build

build-monitoring:
	docker compose -f docker-compose.yml  -f docker-compose.monitoring.yml build

deploy: build down
	docker compose -f docker-compose.yml up -d

deploy-monitoring: build-monitoring down
	docker compose -f docker-compose.yml -f docker-compose.monitoring.yml up -d

down:
	docker compose -f docker-compose.yml -f docker-compose.monitoring.yml down --remove-orphans

# View logs
logs:
	docker compose logs -f

logs-api:
	docker compose logs -f api

logs-frontend:
	docker compose logs -f frontend

# Testing
test:
	go test ./...

show-coverage:
	go test ./... -coverprofile fmtcoverage.html
	go tool cover -html fmtcoverage.html
