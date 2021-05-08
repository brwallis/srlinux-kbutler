#
# Credit:
#   This makefile was adapted from: https://github.com/k8snetworkplumbingwg/sriov-cni/blob/master/Makefile, 
#   which was adapted from: https://github.com/vincentbernat/hellogopher/blob/feature/glide/Makefile
#
# Package related
#REPO_USER=${REPO_USER}
#REPO_PASSWORD=${REPO_PASSWORD}
BINARY_NAME=kbutler
PACKAGE=kbutler
ORG_PATH=srlinux.io
REPO_PATH=$(ORG_PATH)/$(PACKAGE)
GOPATH=$(CURDIR)/.gopath
GOBIN=$(CURDIR)/bin
BUILDDIR=$(CURDIR)/build
#BASE=$(GOPATH)/src/$(REPO_PATH)
BASE=$(CURDIR)
GOFILES = $(shell find . -name *.go | grep -vE "(\/vendor\/)|(_test.go)")
PKGS     = $(or $(PKG),$(shell cd $(BASE) && env GOPATH=$(GOPATH) $(GO) list ./... | grep -v "^$(PACKAGE)/vendor/"))
# Docker
IMAGEDIR=$(BASE)/images
DOCKERFILE=$(CURDIR)/Dockerfile
TAG=srlinux.io/kbutler

export GOPATH
export GOBIN
export GO111MODULE=on
# Accept proxy settings for docker 
DOCKERARGS = --no-cache
ifdef HTTP_PROXY
	DOCKERARGS += --build-arg http_proxy=$(HTTP_PROXY)
endif
ifdef HTTPS_PROXY
	DOCKERARGS += --build-arg https_proxy=$(HTTPS_PROXY)
endif

# Go tools
GO      = go
GODOC   = godoc
GOFMT   = gofmt
TIMEOUT = 15
V = 0
Q = $(if $(filter 1,$V),,@)

.PHONY: all
all: fmt lint build

$(BASE): ; $(info  Setting GOPATH...)
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR) $@

$(GOBIN):
	@mkdir -p $@

$(BUILDDIR): | $(BASE) ; $(info Creating build directory...)
	@cd $(BASE) && mkdir -p $@

.PHONY: build
build: | $(BUILDDIR)/$(BINARY_NAME) ; $(info Building $(BINARY_NAME)...) @ ## Build SR Linux Kubernetes Butler
	$(info Done!)

$(BUILDDIR)/$(BINARY_NAME): $(GOFILES) | $(BUILDDIR)
	@cd $(BASE)/cmd && GOPRIVATE=github.com/brwallis GIT_TERMINAL_PROMPT=1 GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v
#	@cd $(BASE)/cmd && GOPRIVATE=github.com/brwallis/srlinux-go GIT_TERMINAL_PROMPT=1 GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -o $(BUILDDIR)/$(BINARY_NAME) -tags no_openssl -v


# Tools
GOLINT = $(GOBIN)/golint
$(GOBIN)/golint: | $(BASE) ; $(info  Building golint...)
	$Q go get -u golang.org/x/lint/golint


.PHONY: lint
lint: | $(BASE) $(GOLINT) ; $(info  Running golint...) @ ## Run golint on all source files
	$Q cd $(BASE) && ret=0 && for pkg in $(PKGS); do \
		test -z "$$($(GOLINT) $$pkg | tee /dev/stderr)" || ret=1 ; \
	 done ; exit $$ret

.PHONY: fmt
fmt: ; $(info  Running gofmt...) @ ## Run gofmt on all source files
	@ret=0 && for d in $$($(GO) list -f '{{.Dir}}' ./... | grep -v /vendor/); do \
		$(GOFMT) -l -w $$d/*.go || ret=$$? ; \
	 done ; exit $$ret

# Docker image
# To pass proxy for Docker invoke it as 'make image HTTP_POXY=http://192.168.0.1:8080'
.PHONY: image
image: | $(BASE) ; $(info Building SR Linux Kubernetes Butler Docker image...) @ ## Build SR Linux KButler docker image
	@docker build --build-arg REPO_USER=$(REPO_USER) --build-arg REPO_PASSWORD=$(REPO_PASSWORD) -t $(TAG) -f $(DOCKERFILE) $(CURDIR) $(DOCKERARGS)
# Misc

.PHONY: clean
clean: | $(BASE) ; $(info  Cleaning...) @ ## Cleanup everything
	@cd $(BASE) && $(GO) clean --modcache --cache --testcache
	@rm -rf $(GOPATH)
	@rm -rf $(BUILDDIR)
	@rm -rf $(GOBIN)
	@rm -rf test/

.PHONY: help
help: ; @ ## Display this help message
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
