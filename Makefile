VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/harris/gemini-web-cli/pkg/version.Version=$(VERSION) \
	-X github.com/harris/gemini-web-cli/pkg/version.GitCommit=$(COMMIT) \
	-X github.com/harris/gemini-web-cli/pkg/version.BuildDate=$(DATE)

.PHONY: build clean

build:
	go build -ldflags '$(LDFLAGS)' -o bin/gemini-web-cli .

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/gemini-web-cli-darwin-arm64 .

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/gemini-web-cli-darwin-amd64 .

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/gemini-web-cli-linux-arm64 .

build-linux-arm64-static:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS) -extldflags "-static"' -o bin/gemini-web-cli-linux-arm64-static .

build-all: build-darwin-arm64 build-darwin-amd64 build-linux-arm64

clean:
	rm -rf bin/
