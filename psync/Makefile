BINARY  := psync
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build test lint clean install

all: build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/psync

install:
	go install $(LDFLAGS) ./cmd/psync

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)
