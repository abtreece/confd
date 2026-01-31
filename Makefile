.PHONY: build install clean lint test integration dep release
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")
GIT_SHA=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

build:
	@echo "Building confd..."
	@mkdir -p bin
	@go build -ldflags "-X main.Version=$(VERSION) -X main.GitSHA=$(GIT_SHA)" -o bin/confd ./cmd/confd

install:
	@echo "Installing confd..."
	@install -c bin/confd /usr/local/bin/confd

clean:
	@rm -f bin/*

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

test: lint
	@echo "Running tests..."
	@go test `go list ./... | grep -v vendor/`

integration:
	@echo "Running integration tests..."
	@for i in `find ./test/integration -name test.sh`; do \
		echo "Running $$i"; \
		bash $$i || exit 1; \
		bash test/integration/shared/expect/check.sh || exit 1; \
		rm /tmp/confd-*; \
	done

mod:
	@go mod tidy


snapshot:
	@goreleaser release --snapshot --skip=publish --clean

release:
	@goreleaser release --skip=publish --clean --skip=validate
