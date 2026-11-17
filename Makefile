BIN := bin/proxy-antigravity
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet install doctor clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

install: build
	./$(BIN) install

doctor: build
	./$(BIN) doctor

clean:
	rm -rf bin
