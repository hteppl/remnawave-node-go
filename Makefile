# Variables
BINARY_NAME=remnawave-node-go
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/remnawave/node-go/internal/version.Version=$(VERSION)"

.PHONY: all build test clean install run lint

all: build

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/node-go

test:
	go test -v ./...

test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)

run: build
	./$(BINARY_NAME)

lint:
	golangci-lint run ./...

generate-secrets:
	./scripts/generate-test-secrets.sh
