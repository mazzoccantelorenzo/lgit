.PHONY: build run test lint format hooks clean install

build:
	go build -o bin/lgit .

install: build
	sudo mv bin/lgit /usr/local/bin/lgit
	@echo "lgit installed successfully in /usr/local/bin/"

run: build
	./bin/lgit

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
