include golang.mk
include wag.mk

.PHONY: all test build run dynamodb-test
SHELL := /bin/bash
APP_NAME ?= workflow-manager
EXECUTABLE = $(APP_NAME)
PKG = github.com/Clever/$(APP_NAME)
PKGS := $(shell go list ./... | grep -v /vendor | grep -v /gen-go | grep -v /workflow-ops | grep -v /dynamodb)

WAG_VERSION := latest

$(eval $(call golang-version-check,1.9))

all: test build

test: $(PKGS) dynamodb-test
$(PKGS): golang-test-all-deps
	$(call golang-test-all,$@)

dynamodb-test:
	./run_dynamodb_store_test.sh

build:
	CGO_ENABLED=0 go build -installsuffix cgo -o build/$(EXECUTABLE) $(PKG)
	cp ./kvconfig.yml ./build/kvconfig.yml

run: build
	TZ=UTC build/$(EXECUTABLE)

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
	go build -o bin/mockgen    ./vendor/github.com/golang/mock/mockgen
	mkdir -p mocks/mock_dynamodbiface
	rm -f mocks/mock_dynamodbiface/mock_dynamodbapi.go
	bin/mockgen -package mock_dynamodbiface -source ./vendor/github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface/interface.go DynamoDBAPI > mocks/mock_dynamodbiface/mock_dynamodbapi.go

	mkdir -p mocks/mock_sfniface
	rm -f mocks/mock_sfniface/mock_sfnapi.go
	bin/mockgen -package mock_sfniface -source ./vendor/github.com/aws/aws-sdk-go/service/sfn/sfniface/interface.go SFNAPI > mocks/mock_sfniface/mock_sfnapi.go
