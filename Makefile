.PHONY: run test build migrate clean stop

run: stop
	docker compose up -d
	go run cmd/compute-api/main.go

stop:
	@fuser -k 8080/tcp 2>/dev/null || true

test:
	go test ./...

build:
	mkdir -p bin
	go build -o bin/compute-api cmd/compute-api/main.go
	go build -o bin/cloud cmd/cloud-cli/*.go
	go build -o bin/cloud_cli cmd/cloud_cli/main.go

install: build
	mkdir -p $(HOME)/.local/bin
	cp bin/cloud $(HOME)/.local/bin/cloud
	cp bin/cloud_cli $(HOME)/.local/bin/cloud_cli
	@./scripts/setup_path.sh

setup-path:
	@./scripts/setup_path.sh

migrate:
	@echo "Running migrations..."
	# Placeholder for migration command

clean:
	rm -rf bin
	docker compose down
