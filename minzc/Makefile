.PHONY: build test clean run

# Build the compiler
build:
	go build -o minzc cmd/minzc/main.go

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f minzc
	rm -f *.a80

# Build and run on test file
run: build
	./minzc ../examples/simple_add.minz

# Install dependencies
deps:
	go mod download
	go mod tidy