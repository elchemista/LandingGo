GO ?= go
NPM ?= npm
CONFIG ?= config.dev.json
ADDR ?= :8080
BINARY ?= bin/landing

.PHONY: dev pack build test clean fmt assets

assets:
	$(NPM) run build

dev:
	$(GO) run ./cmd/landing --dev --config=$(CONFIG) --addr=$(ADDR)

pack: assets
	$(GO) run ./cmd/landingo pack --config=$(CONFIG) --web=web --build=build

build: assets
	$(GO) run ./cmd/landingo build --config=$(CONFIG) --web=web --build=build --output=$(BINARY) --go=$(GO)

test:
	$(GO) test ./...

clean:
	rm -rf bin
	rm -rf build/public
	rm -f build/embedded.go
	rm -rf node_modules

fmt:
	gofmt -w $$(find . -type f -name '*.go' -not -path './vendor/*')
