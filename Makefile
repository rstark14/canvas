.PHONY: build cover start test test-integration

build:
	docker build -t canvas .
	
.PHONY: cover start test test-integration

cover:
	go tool cover -html=cover.out

start:
	go run cmd/server/main.go

test:
	go test -coverprofile=cover.out -short ./...

test-integration:
	go test -coverprofile=cover.out -p 1 ./...
