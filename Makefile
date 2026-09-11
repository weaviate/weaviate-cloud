.PHONY: build lint test tidy mocks

build:
	go build ./...

mocks:
	mockery

lint:
	golangci-lint run

test:
	go test ./...

tidy:
	go mod tidy
