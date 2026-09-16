.PHONY: build run test lint format hooks clean

build:
	go build -o bin/lorenzogit .

run: build
	./bin/lorenzogit

test:
	go test ./...

lint:
	go vet ./...

format:
	go fmt ./...

hooks:
	git config core.hooksPath .githooks

clean:
	rm -rf bin/
