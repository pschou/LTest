.PHONY: build clean test run

# Binary name
BINARY_NAME=ltest
VERSION = 0.1.$(shell date +%Y%m%d.%H%M)
FLAGS := "-s -w -X main.version=${VERSION}"

# Build the application
build:
	docker image inspect builder &>/dev/null || docker build -t builder .
	docker run -it --rm -v .:/app builder go build -v -ldflags=${FLAGS} -o $(BINARY_NAME) .

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build -o $(BINARY_NAME)-windows-arm64.exe .

# Run tests
test:
	go test -v ./...

# Run with default settings
run:
	./$(BINARY_NAME) -n 3 -t 5000

# Install dependencies
deps:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
