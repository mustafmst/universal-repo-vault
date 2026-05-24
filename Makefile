.PHONY: build test tidy lint

BINARY := dist/urv

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
