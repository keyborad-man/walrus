.PHONY: build build-web build-go test dev-mock docker-build

VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
LDFLAGS = -s -w -X walrus/internal/version.Version=$(VERSION) -X walrus/internal/version.Commit=$(COMMIT) -X walrus/internal/version.BuildTime=$(BUILD_TIME)

build-web:
	cd web && npm run build

build-go:
	go build -trimpath -ldflags="$(LDFLAGS)" -o bin/walrus ./cmd/walrus

build: build-web build-go

test:
	go test -v ./...

dev-mock:
	go run ./cmd/walrus --runtime=mock --dev --data-dir=./data

docker-build:
	docker build -f docker/Dockerfile \
		--build-arg WALRUS_VERSION=$(VERSION) \
		--build-arg WALRUS_COMMIT=$(COMMIT) \
		--build-arg WALRUS_BUILD_TIME=$(BUILD_TIME) \
		-t walrus:latest .
