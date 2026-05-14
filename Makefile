.PHONY: build build-linux build-windows build-all test clean run-cli run-server

# Binary name
BINARY=topo-match

# Build
build:
	go build -o bin/$(BINARY) ./cmd/topo-match/

# Cross compilation
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 ./cmd/topo-match/

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe ./cmd/topo-match/

build-all: build-linux build-windows

# Test
test:
	go test ./... -v

# Clean
clean:
	rm -rf bin/

# Run CLI mode
run-cli: build
	./bin/$(BINARY) --logic examples/logic_topo.xml --testbed examples/testbed.xml

# Run server mode
run-server: build
	./bin/$(BINARY) --serve --port 8080

# Dependencies
deps:
	go mod tidy
