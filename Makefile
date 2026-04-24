BINARY := ffh
INSTALL_PATH := /usr/local/bin/ffh

.PHONY: build install clean test

build:
	go build -o $(BINARY) .

install: build
	sudo install -m755 $(BINARY) $(INSTALL_PATH)

clean:
	rm -f $(BINARY)

test:
	go test ./...
