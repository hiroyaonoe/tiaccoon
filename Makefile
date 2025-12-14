CLEAN = test/server-tcp.out test/client-tcp.out test/server-unix.out test/client-unix.out
BUILDDIR = build
STATICDIR = static
TARGETS = %/cni %/tiaccoon
GOSRC = $(shell find . -type f -name '*.go')
TEST_SRC = $(shell find test -type f -name '*.c')

GO ?= go
PACKAGE := github.com/hiroyaonoe/tiaccoon
VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.m' --always --tags)
VERSION_TRIMMED := $(VERSION:v%=%)
GO_BUILD_FLAGS += -trimpath
GO_BUILD_LDFLAGS += -s -w -X $(PACKAGE)/pkg/version.Version=$(VERSION)
GO_BUILD := $(GO) build $(GO_BUILD_FLAGS) -ldflags "$(GO_BUILD_LDFLAGS)"
GO_BUILD_STATIC := CGO_ENABLED=1 $(GO) build $(GO_BUILD_FLAGS) -tags "netgo osusergo" -ldflags "$(GO_BUILD_LDFLAGS) -extldflags -static"
STRIP ?= strip

CC = gcc

.DEFAULT: all

all: tidy fmt vet test build $(TEST_SRC:.c=.out)
static: tidy fmt vet test build/static

$(BUILDDIR): %: $(TARGETS)

$(BUILDDIR)/$(STATICDIR): %: $(TARGETS)

$(BUILDDIR)/%: cmd/%/main.go $(GOSRC)
	$(GO_BUILD) -o $@ $<

$(BUILDDIR)/$(STATICDIR)/%: cmd/%/main.go $(GOSRC)
	$(GO_BUILD_STATIC) -o $@ $<
	$(STRIP) $@

.PHONY: tidy
tidy: go.sum go.mod $(GOSRC)
	go mod tidy

.PHONY: fmt
fmt: $(GOSRC)
	$(GO) fmt ./...

.PHONY: vet
vet: $(GOSRC)
	$(GO) vet ./...

.PHONY: test
test: $(GOSRC)
	$(GO) test ./...
	
.PHONY: install
install: $(BUILDDIR)/cni
	install $^ /opt/cni/bin/tiaccoon

.PHONY: uninstall
uninstall:
	- $(RM) /opt/cni/bin/tiaccoon

$(TEST_SRC:.c=.out): %.out: %.c
	$(CC) -o $@ $<

.PHONY: clean
clean:
	- $(RM) -rf $(BUILDDIR)
	- $(RM) $(CLEAN)
