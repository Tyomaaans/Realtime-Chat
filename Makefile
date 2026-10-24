.PHONY: install backend frontend check check-backend check-frontend browser load

install:
	cd backend && go mod download
	cd frontend && npm ci

backend:
	cd backend && go run .

frontend:
	cd frontend && npm run dev

check: check-backend check-frontend

check-backend:
	cd backend && test -z "$$(gofmt -l .)"
	cd backend && go vet ./...
	cd backend && go test ./...
	cd backend && go test -race ./...
	cd backend && go build ./...

check-frontend:
	cd frontend && npm run lint
	cd frontend && npm run typecheck
	cd frontend && npm run test
	cd frontend && npm run build

browser:
	cd frontend && npm run test:browser

LOAD_ARGS ?= -sockets 100 -duration 30s
load:
	cd backend && go run ../scripts/loadtest/main.go $(LOAD_ARGS)
