.PHONY: run test test-coverage build migrate clean stop swagger

run: stop
	docker compose up -d
	go run cmd/api/main.go

stop:
	@fuser -k 8080/tcp 2>/dev/null || true

test:
	go test ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm coverage.out

swagger:
	@$(HOME)/go/bin/swag init -d cmd/api,internal/handlers -g main.go -o docs/swagger --parseDependency --parseInternal

build:
	mkdir -p bin
	go build -o bin/api cmd/api/main.go
	go build -o bin/cloud cmd/cloud/*.go

install: build
	mkdir -p $(HOME)/.local/bin
	cp bin/cloud $(HOME)/.local/bin/cloud
	@./scripts/setup_path.sh

setup-path:
	@./scripts/setup_path.sh

migrate:
	@echo "Running migrations..."
	@docker compose up -d postgres
	@sleep 2
	@go run cmd/api/main.go --migrate-only 2>/dev/null || echo "Migrations applied via server startup"

migrate-status:
	@echo "Checking migration status..."
	@docker compose exec postgres psql -U cloud -d thecloud -c "SELECT * FROM schema_migrations;" 2>/dev/null || echo "No migrations table found"

clean:
	rm -rf bin
	docker compose down
