BINARY   := vmcore-checker
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOFLAGS  := -trimpath -buildvcs=false
LDFLAGS  := -s -w -X main.version=$(VERSION)
TINYGO   ?= tinygo

# Size budgets for the initramfs, enforced by `make size-check` and
# `make tiny-size-check`: 2.2 MiB for the stock Go build, 1 MiB for the
# TinyGo build.
MAX_SIZE_BYTES      := 2306867
TINY_MAX_SIZE_BYTES := 1048576

.PHONY: build test linux-amd64 linux-arm64 size-check tiny-size-check verify-tiny clean

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

# TinyGo builds: ~3x smaller than the stock Go build. -scheduler=none and
# -gc=leaking were measured to save nothing / fail to link respectively
# (TinyGo 0.41.1), so plain -opt=z -no-debug it is. Gate behavior with
# `make verify-tiny` before shipping these into a crash kernel.
tiny-linux-%:
	GOOS=linux GOARCH=$* $(TINYGO) build -opt=z -no-debug -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-tiny-linux-$* .

tiny-size-check: tiny-linux-amd64 tiny-linux-arm64
	@for f in $(BINARY)-tiny-linux-amd64 $(BINARY)-tiny-linux-arm64; do \
		size=$$(wc -c < $$f); \
		if [ $$size -gt $(TINY_MAX_SIZE_BYTES) ]; then \
			echo "FAIL: $$f is $$size bytes (budget $(TINY_MAX_SIZE_BYTES))"; exit 1; \
		fi; \
		echo "OK: $$f is $$size bytes (budget $(TINY_MAX_SIZE_BYTES))"; \
	done

# Behavioral parity gate between the stock Go and TinyGo compilers.
verify-tiny:
	scripts/verify-tiny.sh

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 $(BINARY)-tiny-linux-amd64 $(BINARY)-tiny-linux-arm64
