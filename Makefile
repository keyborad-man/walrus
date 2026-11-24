.PHONY: build build-web build-go test dev-mock docker-build

build-web:
	cd web && npm run build

build-go:
	go build -trimpath -o bin/walrus ./cmd/walrus

build: build-web build-go

test:
	go test -v ./...

dev-mock:
	go run ./cmd/walrus --runtime=mock --dev --data-dir=./data

docker-build:
	docker build -f docker/Dockerfile -t walrus:latest .
