.PHONY: install run build vet format format-check test test-critical load

export GOCACHE := $(CURDIR)/.cache/go-build

LOAD_ARGS ?= -sockets 100 -duration 30s

install:
	go mod download

run:
	go run .

build:
	mkdir -p bin
	go build -o bin/realtime-chat .
	go build -o bin/load-test ./scripts/load-test

vet:
	go vet ./...

format:
	gofmt -w main.go internal pkg scripts

format-check:
	test -z "$$(gofmt -l main.go internal pkg scripts)"

test:
	go test ./...

test-critical:
	go test -race ./internal/chat/session -run 'Test(ParseCommand|MessageValidation|FIFOQueue|Next|Relay|FullBuffers|RealClients|OriginsCapacity|ConcurrentNext|ShutdownCloses)'
	go test -race ./internal/middleware ./pkg -run 'Test(ConcurrentReservations|TokenBucket)'

load:
	go run ./scripts/load-test $(LOAD_ARGS)
