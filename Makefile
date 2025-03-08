GHCR_REPO:=ghcr.io/babbage88/goinfacli:
BIN_NAME:=goinfracli
MAIN_BRANCH:=master
VERSION_TYPE:=patch
INSTALL_PATH:=$${GOPATH}/bin
ENV_FILE:=.env
MIG:=$(shell date '+%m%d%Y.%H%M%S')
SHELL := /bin/bash
VERBOSE ?= 1
ifeq ($(VERBOSE),1)
	V = -v
endif
export LATEST_TAG := $(shell git fetch --tags && git tag -l "v[0-9]*.[0-9]*.[0-9]*" | sort -V | tail -n 1)

sqlc-and-migrations:
	source config_goose.sh
	goose down -v
	goose up -v
	sqlc generate

build:
	go build $(V) -o $(BIN_NAME) .

build-quiet:
	go build -o goinfracli

install: build
	echo "Install Path $(INSTALL_PATH)"
	mv $(BIN_NAME) $(INSTALL_PATH)

# Add this target to the end of your Makefile

# Usage: make release [VERSION=major|minor|patch]
fetch-tags:
	@{ \
	  branch=$$(git rev-parse --abbrev-ref HEAD); \
	  if [ "$$branch" != "$(MAIN_BRANCH)" ]; then \
	    echo "Error: You must be on the $(MAIN_BRANCH) branch. Current branch is '$$branch'."; \
	    exit 1; \
	  fi; \
	  git fetch origin $(MAIN_BRANCH); \
	  UPSTREAM=origin/$(MAIN_BRANCH); \
	  LOCAL=$$(git rev-parse @); \
	  REMOTE=$$(git rev-parse "$$UPSTREAM"); \
	  BASE=$$(git merge-base @ "$$UPSTREAM"); \
	  if [ "$$LOCAL" != "$$REMOTE" ]; then \
	    echo "Error: Your local $(MAIN_BRANCH) branch is not up-to-date with remote. Please pull the latest changes."; \
	    exit 1; \
	  fi; \
	  git fetch --tags; \
	}

release: fetch-tags
	@{ \
	  echo "Latest tag: $(LATEST_TAG)"; \
	  new_tag=$$(go run . utils version-bumper --latest-version "$(LATEST_TAG)" --increment-type=$(VERSION_TYPE)); \
	  echo "Creating new tag: $$new_tag"; \
	  git tag -a $$new_tag -m $$new_tag && git push --tags; \
	}


check-builder:
	@if ! docker buildx inspect goinfaclibuilder > /dev/null 2>&1; then \
		echo "Builder goinfaclibuilder does not exist. Creating..."; \
		docker buildx create --name goinfaclibuilder --bootstrap; \
	fi

create-builder: check-builder

buildandpush: check-builder
	docker buildx use goinfaclibuilder
	docker buildx build --platform linux/amd64,linux/arm64 -t $(GHCR_REPO)$(tag) . --push

deploydev: buildandpushdev
	kubectl apply -f deployment/kubernetes/infra-goinfacli.yaml
	kubectl rollout restart deployment infra-goinfacli

