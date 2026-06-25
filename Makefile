.PHONY: dev-backend dev-frontend build test lint migrate-up migrate-down hooks

dev-backend:
	@which air > /dev/null 2>&1 && air -c .air.toml || go run ./backend/cmd/server

dev-frontend:
	npm --prefix web run dev

build:
	npm --prefix web run build
	(cd backend && go build -o ../bin/wiselabz ./cmd/server)

test:
	go test -short ./backend/...

lint:
	golangci-lint run ./backend/...
	npm --prefix web run lint

migrate-up:
	(cd backend && go run ./cmd/migrate up)

migrate-down:
	(cd backend && go run ./cmd/migrate down)

hooks:
	lefthook install
