.PHONY: tidy fmt vet build test

tidy:
	go mod tidy

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

build:
	go build ./cmd/releaser

test:
	go test ./...