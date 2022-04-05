#Client load balance Makefile

BINARY_NAME=clientLB
GOBIN=$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build
BASE=$(CURDIR)
GOFILES = $(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")

# Go tools
GO      = go
GODOC   = godoc
GOFMT   = gofmt
TIMEOUT = 15
V = 0
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: build

$(GOBIN):
	@mkdir -p $@

$(BUILDDIR): | $(BASE) ; $(info Creating build directory...)
	@cd $(BASE) && mkdir -p $@

build: $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) @ ## Build Client Load Balance Binary
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd $(BASE) && CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v

.PHONY: clean
clean: | $(BASE) ; $(info  Cleaning...) @ ## Cleanup everything
	@cd $(BASE) && $(GO) clean --modcache --cache --testcache
	@rm -rf $(GOPATH)
	@rm -rf $(BUILDDIR)
	@rm -rf $(GOBIN)
	@rm -rf test/
	$(info Done!)

.PHONY: help
help: ; @ ## Display this help message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'