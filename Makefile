include golang.mk
include lambda.mk
include wag.mk

SHELL := /bin/bash
APP_NAME ?= workflow-manager
EXECUTABLE = $(APP_NAME)
PKG = github.com/Clever/$(APP_NAME)
PKGS := $(shell go list ./... | grep -v /vendor | grep -v /gen-go | grep -v /workflow-ops | grep -v /dynamodb)
PKGS := $(PKGS) $(PKG)/gen-go/server/db/dynamodb
APPS := $(shell [ -d "./cmd" ] && ls ./cmd/)

.PHONY: all test build run dynamodb-test $(APPS) $(APP_NAME)

WAG_VERSION := latest

$(eval $(call golang-version-check,1.16))

all: test build

test: $(PKGS) dynamodb-test

$(PKGS): golang-test-all-deps
	$(call golang-test-all,$@)

dynamodb-test:
	./run_dynamodb_store_test.sh

$(APPS):
	$(call lambda-build-go,./cmd/$@,$@)

$(APP_NAME):
	$(call golang-build,$(PKG),$(EXECUTABLE))

build: $(APP_NAME) $(APPS)

# Local development
#
# When you run e.g. `ark start -l <app>` ark will inject _APP_NAME=<app>
# This allows us to rebuild only the relevant app's code.
# when we build locally we don't suffix with the region
build-local:
ifeq ($(_APP_NAME),workflow-manager)
	$(call golang-build,$(PKG),$(_APP_NAME))
else
	$(call golang-build,./cmd/$(_APP_NAME),$(_APP_NAME))
endif

run: build-local
ifeq ($(_APP_NAME),workflow-manager)
	TZ=UTC bin/$(_APP_NAME)
else
	LOCAL=true KAYVEE_LOG_LEVEL=debug bin/$(_APP_NAME)
endif

run-docker:
	@docker run \
	--env-file=<(echo -e $(_ARKLOC_ENV_FILE)) clever/workflow-manager:569f2dc

swagger2markup-cli-1.3.1.jar:
	curl -L -O https://jcenter.bintray.com/io/github/swagger2markup/swagger2markup-cli/1.3.1/$@

generate: wag-generate-deps swagger2markup-cli-1.3.1.jar
	java -jar swagger2markup-cli-1.3.1.jar convert -c docs/config.properties -i swagger.yml  -d docs/
	$(call wag-generate-mod,./swagger.yml)
	# wag bug: this test file assumes usage of functions like `aws.String(...)` but the workflow-manager
	# ark db doesn't use this, leading to an unused import error. Remove the import with a hack.
	sed -i '6d' gen-go/server/db/tests/tests.go
	bin/launch-gen -o ./cmd/sfn-execution-events-consumer/launch.go -p main launch/sfn-execution-events-consumer.yml

install_deps:
	go mod vendor
	go build -o bin/mockgen ./vendor/github.com/golang/mock/mockgen
	go build -o bin/launch-gen github.com/Clever/launch-gen
	rm -rf mocks/mock_*.go
	for svc in dynamodb sfn sqs cloudwatchlogs; do \
	  bin/mockgen -package mocks -source ./vendor/github.com/aws/aws-sdk-go/service/$${svc}/$${svc}iface/interface.go -destination mocks/mock_$${svc}.go; \
	done
	bin/mockgen -package mocks -source ./executor/workflow_manager.go -destination mocks/mock_workflow_manager.go WorkflowManager
	bin/mockgen -package mocks -source ./store/store.go -destination mocks/mock_store.go Store
