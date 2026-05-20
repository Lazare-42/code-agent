BINARY := code-agent
PKG := ./cmd/code-agent

.PHONY: build install test fmt lint clean

build:
	go build -o $(BINARY) $(PKG)

install:
	go install $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
