include golang.mk
include wag.mk

.PHONY: all test build run dynamodb-test
SHELL := /bin/bash
APP_NAME ?= workflow-manager
EXECUTABLE = $(APP_NAME)
PKG = github.com/Clever/$(APP_NAME)
PKGS := $(shell go list ./... | grep -v /vendor | grep -v /gen-go | grep -v /workflow-ops | grep -v /dynamodb)

WAG_VERSION := latest

$(eval $(call golang-version-check,1.8))

all: test build

test: $(PKGS) dynamodb-test
$(PKGS): golang-test-all-deps
	$(call golang-test-all,$@)

dynamodb-test:
	./run_dynamodb_store_test.sh

build:
	CGO_ENABLED=0 go build -installsuffix cgo -o build/$(EXECUTABLE) $(PKG)

run: build
	build/$(EXECUTABLE)

run-docker:
	@docker run \
	--env-file=<(echo -e $(_ARKLOC_ENV_FILE)) clever/workflow-manager:569f2dc

generate: wag-generate-deps
	$(call wag-generate,./swagger.yml,$(PKG))

$(GOPATH)/bin/glide:
	@go get github.com/Masterminds/glide

install_deps: $(GOPATH)/bin/glide
	@$(GOPATH)/bin/glide install -v
