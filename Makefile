APP_NAME := coco-ext
VERSION := v0.1.0
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +%Y-%m-%d)
LDFLAGS := -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build build-all build-ui test clean install

build-ui:
	@if ! command -v npm >/dev/null 2>&1; then \
		echo "npm 未安装，无法构建内嵌 Web UI 资源"; \
		exit 1; \
	fi
	cd web && npm run build

build: build-ui
	go build -ldflags "$(LDFLAGS)" -o $(APP_NAME) .

build-all: build-ui
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_darwin_arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_linux_amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP_NAME)_linux_arm64 .

test:
	go test ./... -v

clean:
	rm -f $(APP_NAME)
	rm -rf dist/

install: build
	@mkdir -p $(HOME)/.local/bin
	mv $(APP_NAME) $(HOME)/.local/bin/
