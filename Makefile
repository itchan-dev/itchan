.PHONY: down dev test show-coverage deploy deploy-monitoring logs logs-frontend logs-api gen-configs

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

gen-configs:
	./scripts/gen-configs.sh

deploy: gen-configs
	docker compose -f docker-compose.yml up -d --build --remove-orphans

deploy-monitoring: gen-configs
	docker compose -f docker-compose.yml -f docker-compose.monitoring.yml up -d --build --remove-orphans

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
