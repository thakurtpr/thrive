.PHONY: build test lint clean install

build:
	go build -o bin/thrive ./cmd/thrive

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

install: build
	cp bin/thrive /usr/local/bin/thrive
