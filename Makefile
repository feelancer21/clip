.PHONY: build install

VERSION := $(shell git rev-parse --short HEAD)
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o clip-cli ./cmd/clip-cli

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/clip-cli
