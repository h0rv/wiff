default: all

version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# Format code
fmt:
    gofmt -w .

# Run go vet
vet:
    go vet ./...

# Run tests
test:
    go test ./...

# Run golangci-lint (if installed)
lint:
    @command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed, skipping"

# Build binary
build:
    go build -ldflags "-X main.version={{version}}" -o wiff .

install:
    go install -ldflags "-X main.version={{version}}" .

# Run all checks
all: fmt vet lint test build

# Clean build artifacts
clean:
    rm -f wiff
