include golang.mk
include wag.mk

.PHONY: all test build run dynamodb-test
SHELL := /bin/bash
APP_NAME ?= workflow-manager
EXECUTABLE = $(APP_NAME)
PKG = github.com/Clever/$(APP_NAME)
PKGS := $(shell go list ./... | grep -v /vendor | grep -v /gen-go | grep -v /workflow-ops | grep -v /dynamodb)
PKGS := $(PKGS) $(PKG)/gen-go/server/db/dynamodb

WAG_VERSION := latest

$(eval $(call golang-version-check,1.13))

all: test build

test: $(PKGS) dynamodb-test
$(PKGS): golang-test-all-deps
	$(call golang-test-all,$@)

dynamodb-test:
	./run_dynamodb_store_test.sh

build:
	$(call golang-build,$(PKG),$(EXECUTABLE))
	cp ./kvconfig.yml ./bin/kvconfig.yml

run: build
	TZ=UTC bin/$(EXECUTABLE)

run-docker:
	@docker run \
	--env-file=<(echo -e $(_ARKLOC_ENV_FILE)) clever/workflow-manager:569f2dc

swagger2markup-cli-1.3.1.jar:
	curl -L -O https://jcenter.bintray.com/io/github/swagger2markup/swagger2markup-cli/1.3.1/$@

generate: wag-generate-deps swagger2markup-cli-1.3.1.jar
	java -jar swagger2markup-cli-1.3.1.jar convert -c docs/config.properties -i swagger.yml  -d docs/
	$(call wag-generate,./swagger.yml,$(PKG))

install_deps: golang-dep-vendor-deps
	$(call golang-dep-vendor)
	go build -o bin/mockgen ./vendor/github.com/golang/mock/mockgen
	cp bin/mockgen $(GOPATH)/bin/mockgen
	# hack to workaround dep not working with go module suffixes https://github.com/golang/dep/issues/2139
	if [ ! -f vendor/github.com/elastic/go-elasticsearch/v6 ]; then \
          mkdir -p vendor/github.com/elastic/go-elasticsearch/ && \
          git clone git@github.com:elastic/go-elasticsearch vendor/github.com/elastic/go-elasticsearch/v6 && \
          cd vendor/github.com/elastic/go-elasticsearch/v6 && \
          git checkout 6.x; \
        fi
	mkdir -p mocks/
	rm -rf mocks/*
	for svc in dynamodb sfn sqs; do \
	  bin/mockgen -package mocks -source ./vendor/github.com/aws/aws-sdk-go/service/$${svc}/$${svc}iface/interface.go -destination mocks/$${svc}.go; \
	done
	bin/mockgen -package mocks -source ./executor/workflow_manager.go -destination mocks/workflow_manager.go WorkflowManager
	bin/mockgen -package mocks -source ./store/store.go -destination mocks/store.go Store
