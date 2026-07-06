BINARY  = konsensomat
IMAGE   = konsensomat
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -ldflags="-X main.version=$(VERSION)"

.PHONY: build build-windows build-rpi64 build-rpi32 build-mac test run docker-build docker-build-alpine up clean

## Build the Linux amd64 binary (used by the Docker image)
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY) .

## Build the Windows amd64 binary
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY).exe .

## Build the Raspberry Pi arm64 binary (Pi 3 / 4 / 5 — 64-bit OS)
build-rpi64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-arm64 .

## Build the Raspberry Pi armv7 binary (Pi 2 / 32-bit OS)
build-rpi32:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o $(BINARY)-armv7 .

## Build the macOS Apple Silicon (arm64) binary
build-mac:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .

## Run all tests
test:
	go test ./...

## Run the application locally without Docker
run:
	go run .

## Build the Linux binary and create the minimal (scratch) Docker image
docker-build: build
	docker build -t $(IMAGE):latest .

## Build the Linux binary and create the Alpine-based Docker image (adds HEALTHCHECK)
docker-build-alpine: build
	docker build -f Dockerfile.alpine -t $(IMAGE):latest-alpine .

## Start the application via Docker Compose (requires: make docker-build)
up:
	docker compose up

## Remove compiled binaries
clean:
	-rm -f $(BINARY) $(BINARY).exe $(BINARY)-arm64 $(BINARY)-armv7 $(BINARY)-darwin-arm64
