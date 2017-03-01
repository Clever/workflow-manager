include golang.mk
include wag.mk

.PHONY: all test build run
SHELL := /bin/bash
APP_NAME ?= workflow-manager
EXECUTABLE = $(APP_NAME)
PKG = github.com/Clever/$(APP_NAME)
PKGS := $(shell go list ./... | grep -v /vendor | grep -v /gen-go)

WAG_VERSION := latest

$(eval $(call golang-version-check,1.7))

all: test build

test: $(PKGS)
$(PKGS): golang-test-all-strict-deps
	$(call golang-test-all-strict,$@)

build:
	CGO_ENABLED=0 go build -installsuffix cgo -o build/$(EXECUTABLE) $(PKG)

run: build
	build/$(EXECUTABLE)

generate: wag-generate-deps
	$(call wag-generate,./swagger.yml,$(PKG))

$(GOPATH)/bin/glide:
	@go get github.com/Masterminds/glide

install_deps: $(GOPATH)/bin/glide
	@$(GOPATH)/bin/glide install -v
