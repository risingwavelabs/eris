.DEFAULT_GOAL       := help
VERSION             := v0.0.0
TARGET_MAX_CHAR_NUM := 20

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)

PROJECT_DIR=$(shell pwd)

.PHONY: help build fmt lint test release-tag release-push

## Show help
help:
	@echo 'Package eris provides a better way to handle errors in Go.'
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
		helpMessage = match(lastLine, /^## (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 3, RLENGTH); \
			printf "  ${YELLOW}%-$(TARGET_MAX_CHAR_NUM)s${RESET} ${GREEN}%s${RESET}\n", helpCommand, helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

## Build the code
build:
	@echo Building
	@go build -v .

## Format with go-fmt
fmt:
	@echo Formatting
	@go fmt .

## Lint with golangci-lint
lint: golangci-lint
	@echo Linting
	@$(PROJECT_DIR)/bin/golangci-lint run --config $(PROJECT_DIR)/.golangci.yaml

golangci-lint:
	GOBIN=$(PROJECT_DIR)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.0

## Format docs
docs:
	@echo Formatting docs
	@npm list -g markdown-toc > /dev/null 2>&1 || npm install -g markdown-toc > /dev/null 2>&1
	@markdown-toc -i README.md

## Run the tests
test:
	@echo Running tests
	@go test -race -covermode=atomic -coverprofile=cover.out -v .
	@go tool cover -html cover.out -o cover.html

tests: test

## Run benchmark tests
bench:
	@echo Running benchmark tests
	@cd benchmark && go test -benchmem -bench=. && cd ..

## Run the tests with coverage
test-coverage:
	@echo Running tests with coverage
	@go test -short -coverprofile cover.out -covermode=atomic ${PKG_LIST}

## Display test coverage
display-coverage: test-coverage
	@echo Displaying test coverage
	@go tool cover -html=cover.out

## Stage a release (usage: make release-tag VERSION={VERSION_TAG})
release-tag: build fmt lint test
	@echo Tagging release with version "${VERSION}"
	@git tag -a ${VERSION} -m "chore: release version '${VERSION}'"
	@echo Generating changelog
	@git-chglog -o CHANGELOG.md
	@git add CHANGELOG.md
	@git commit -m "chore: update changelog for version '${VERSION}'"

## Push a release (warning: make sure the release was staged properly before doing this)
release-push:
	@echo Publishing release
	@git push --follow-tags

spellcheck:
	@if command -v cspell > /dev/null 2>&1 ; then \
	    cspell lint -c hack/cspell/cspell.json --relative --no-progress --no-summary --show-suggestions -e 'vendor/*' --gitignore **/*.go **/*.md; \
	else \
		echo "ERROR: cspell not found, install it manually! Link: https://cspell.org/docs/getting-started"; \
		exit 1; \
	fi
