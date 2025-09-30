GO ?= go
CONFIG ?= config.example.json
ADDR ?= :8080
BINARY ?= bin/landing

.PHONY: dev pack build test clean fmt

dev:
	$(GO) run ./cmd/landing --dev --config=$(CONFIG) --addr=$(ADDR)

pack:
	$(GO) run ./cmd/pack --config=$(CONFIG) --web=web --build=build

build: pack
	@mkdir -p $(dir $(BINARY))
	$(GO) build -ldflags "-s -w" -o $(BINARY) ./cmd/landing

test:
	$(GO) test ./...

clean:
	rm -rf bin
	rm -rf build/public
	rm -f build/embedded.go

fmt:
	gofmt -w $$(find . -type f -name '*.go' -not -path './vendor/*')
