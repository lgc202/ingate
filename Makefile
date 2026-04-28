.PHONY: build test

build:
	mkdir -p _output
	GOCACHE=$(CURDIR)/.gocache go build -o _output/ingate ./cmd/ingate

test:
	GOCACHE=$(CURDIR)/.gocache go test ./...
