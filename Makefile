.PHONY: install backend frontend build check test-critical test browser load format

install:
	$(MAKE) -C backend install
	cd frontend && npm ci

backend:
	$(MAKE) -C backend run

frontend:
	cd frontend && npm run dev

build:
	$(MAKE) -C backend build
	cd frontend && npm run build

check:
	$(MAKE) -C backend format-check vet
	cd frontend && npm run format:check && npm run lint && npm run build

test-critical:
	$(MAKE) -C backend test-critical
	cd frontend && npm run test:critical

test:
	$(MAKE) -C backend test
	cd frontend && npm run test

browser:
	cd frontend && npm run test:browser

LOAD_ARGS ?= -sockets 100 -duration 30s
load:
	$(MAKE) -C backend load LOAD_ARGS="$(LOAD_ARGS)"

format:
	$(MAKE) -C backend format
	cd frontend && npm run format
