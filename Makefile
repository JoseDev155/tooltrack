APP_NAME := tooltrack
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DIST_DIR := dist
CLI_SRC  := ./cmd/cli
LDFLAGS  := -s -w -X 'main.Version=$(VERSION)'

.PHONY: all run test vet fmt fmt-check check clean \
	cli-all cli-linux cli-windows cli-darwin cli-darwin-amd64 cli-darwin-arm64

all: cli-all

run:
	go run $(CLI_SRC)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Fails when any file is not gofmt-formatted.
fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed on:"; echo "$$out"; exit 1; fi

check: fmt-check vet test

clean:
	rm -rf $(DIST_DIR)

# Static binaries (no C runtime) for every supported platform.
cli-linux:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 $(CLI_SRC)

cli-windows:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe $(CLI_SRC)

cli-darwin-amd64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 $(CLI_SRC)

cli-darwin-arm64:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 $(CLI_SRC)

cli-darwin: cli-darwin-amd64 cli-darwin-arm64

cli-all: cli-linux cli-windows cli-darwin
