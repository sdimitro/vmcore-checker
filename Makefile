BINARY   := vmcore-checker
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOFLAGS  := -trimpath
LDFLAGS  := -s -w -X main.version=$(VERSION)

# Size budget for the initramfs (2.5 MiB), enforced by `make size-check`.
MAX_SIZE_BYTES := 2621440

.PHONY: build test linux-amd64 linux-arm64 size-check clean

build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	gofmt -l . | (! grep .)
	go vet ./...
	go test ./...

linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 .

linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-arm64 .

size-check: linux-amd64 linux-arm64
	@for f in $(BINARY)-linux-amd64 $(BINARY)-linux-arm64; do \
		size=$$(wc -c < $$f); \
		if [ $$size -gt $(MAX_SIZE_BYTES) ]; then \
			echo "FAIL: $$f is $$size bytes (budget $(MAX_SIZE_BYTES))"; exit 1; \
		fi; \
		echo "OK: $$f is $$size bytes (budget $(MAX_SIZE_BYTES))"; \
	done

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-linux-arm64
