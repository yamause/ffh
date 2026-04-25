BINARY := ffh
INSTALL_PATH := /usr/local/bin/ffh
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install clean test

build:
	go build $(LDFLAGS) -o $(BINARY) .

install: build
	sudo install -m755 $(BINARY) $(INSTALL_PATH)

clean:
	rm -f $(BINARY)

test:
	go test ./...
