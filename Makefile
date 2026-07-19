.PHONY: build test lint clean run

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o claude-status-line-go ./cmd/claude-status-line-go

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

clean:
	rm -f claude-status-line-go
	rm -f coverage.out
	rm -rf ./claude-status-line-go_*.tar.gz

run: build
	./claude-status-line-go
