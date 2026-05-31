.PHONY: build run test test-race cover vet fmt tidy docker docker-run clean

build:
	go build -trimpath -ldflags="-s -w" -o bin/api ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

docker:
	docker build -t computer-use:latest .

docker-run:
	docker run --rm -p 8080:8080 -e API_KEY=$(API_KEY) computer-use:latest

clean:
	rm -rf bin coverage.out
