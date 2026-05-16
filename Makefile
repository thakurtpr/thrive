.PHONY: build test test-cover race vet lint clean install \
        build-all build-linux build-darwin build-windows test-desktop \
        build-vm-image install-linuxkit

GOOS ?= linux
BINARY := bin/thrive

build:
	GOOS=$(GOOS) go build -o $(BINARY) ./cmd/thrive

# Cross-compile the thrive CLI for every supported host platform.
# Output: bin/thrive-<os>-<arch>[.exe]
# Note: darwin amd64 cross-build from arm64 requires native CGO + macOS SDK
# for the systray tray library; build it natively on an Intel Mac instead.
build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux  GOARCH=amd64 go build -o bin/thrive-linux-amd64 ./cmd/thrive
	GOOS=linux  GOARCH=arm64 go build -o bin/thrive-linux-arm64 ./cmd/thrive
	GOOS=linux  GOARCH=amd64 go build -o bin/thrived-linux-amd64 ./cmd/thrived
	GOOS=linux  GOARCH=arm64 go build -o bin/thrived-linux-arm64 ./cmd/thrived

build-darwin:
	GOOS=darwin GOARCH=arm64 go build -o bin/thrive-darwin-arm64 ./cmd/thrive

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/thrive-windows-amd64.exe ./cmd/thrive
	GOOS=windows GOARCH=arm64 go build -o bin/thrive-windows-arm64.exe ./cmd/thrive

# Desktop-subsystem-only test on the host platform (no GOOS override).
test-desktop:
	go test -count=1 ./internal/vm/...

test:
	GOOS=$(GOOS) go test -v -count=1 ./...

test-cover:
	GOOS=$(GOOS) go test -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt
	go tool cover -html=coverage.txt -o coverage.html

race:
	GOOS=$(GOOS) go test -race -count=1 ./...

vet:
	GOOS=$(GOOS) go vet ./...

lint:
	golangci-lint run --build-tags linux

clean:
	rm -rf bin/ coverage.txt coverage.html

install: build
	cp $(BINARY) /usr/local/bin/thrive

build-vm-image:
	@echo "==> Building VM image (requires linuxkit + docker buildx + linux/arm64 support)"
	bash scripts/build-vm-image.sh arm64

install-linuxkit:
	go install github.com/linuxkit/linuxkit/src/cmd/linuxkit@latest
