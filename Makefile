BINARY  := grove
ALIAS   := wt
PREFIX  ?= $(HOME)/.local/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/PrerakGada/grove/cmd.Version=$(VERSION)

.PHONY: build install install-alias uninstall test vet cross clean

build:
	@go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) .
	@echo "built bin/$(BINARY) ($(VERSION))"

## install links `grove` into PREFIX. The short `wt` name is a separate target
## because it collides with the bash predecessor's shell alias — see README.
install: build
	@mkdir -p $(PREFIX)
	@ln -sf $(CURDIR)/bin/$(BINARY) $(PREFIX)/$(BINARY)
	@echo "linked $(PREFIX)/$(BINARY) -> $(CURDIR)/bin/$(BINARY)"

install-alias: install
	@ln -sf $(CURDIR)/bin/$(BINARY) $(PREFIX)/$(ALIAS)
	@echo "linked $(PREFIX)/$(ALIAS) -> $(CURDIR)/bin/$(BINARY)"
	@echo "NOTE: a shell alias named '$(ALIAS)' shadows this. Remove it from ~/.zshrc first."

uninstall:
	@rm -f $(PREFIX)/$(BINARY) $(PREFIX)/$(ALIAS)
	@echo "removed $(PREFIX)/$(BINARY) and $(PREFIX)/$(ALIAS)"

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
