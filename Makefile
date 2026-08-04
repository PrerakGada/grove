BINARY  := grove
PREFIX  ?= $(HOME)/.local/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/PrerakGada/grove/cmd.Version=$(VERSION)

.PHONY: build install uninstall test vet cross clean

build:
	@go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .
	@echo "built bin/$(BINARY) ($(VERSION))"

install: build
	@mkdir -p $(PREFIX)
	@ln -sf $(CURDIR)/bin/$(BINARY) $(PREFIX)/$(BINARY)
	@echo "linked $(PREFIX)/$(BINARY) -> $(CURDIR)/bin/$(BINARY)"

uninstall:
	@rm -f $(PREFIX)/$(BINARY)
	@echo "removed $(PREFIX)/$(BINARY)"

test:
	@go test ./...

vet:
	@go vet ./...

## cross proves the platform stubs still compile everywhere.
cross:
	@for os in darwin linux windows; do \
		GOOS=$$os GOARCH=amd64 go build -o /dev/null . && echo "  $$os ok"; \
	done

clean:
	@rm -rf bin dist
