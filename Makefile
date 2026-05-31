.PHONY: build test tidy lint

BINARY_NAME := urv

BINARY := dist/$(BINARY_NAME)

build:
	mkdir -p dist
	go build -o $(BINARY) ./main.go

test:
	go test ./...

tidy:
	go mod download
	go mod tidy

lint:
	gofmt -w .
	go vet ./...

install: 
	rm -rf ~/.local/bin/$(BINARY_NAME)
	cp $(BINARY) ~/.local/bin/$(BINARY_NAME)
