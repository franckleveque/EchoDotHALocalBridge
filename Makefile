BINARY_NAME=bridge

.PHONY: all build test clean docker coverage

all: test build

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o $(BINARY_NAME) ./cmd/bridge

test:
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out

docker:
	docker build -t hue-bridge-emulator:latest .
