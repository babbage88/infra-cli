GHCR_REPO:=ghcr.io/babbage88/infractl:
DBHELPERAPI_GHCR_REPO:=ghcr.io/babbage88/dbhelperapi:
ARTIFACT_DIR=dist
BIN_NAME:=infractl
ARTIFACT:=$(ARTIFACT_DIR)/$(BIN_NAME)
DEFAULT_CFG_FILE:=default.yaml
DEFUALT_CONFIG_DIR:=~/.config/infractl
MAIN_BRANCH:=master
VERSION_TYPE:=patch
#INSTALL_PATH:=$${GOPATH}/bin
export CUR_USER:=$(shell whoami)
GOPATH:=$(HOME)/go
GOBIN:=$(GOPATH)/bin
INSTALL_PATH:=$(HOME)/go/bin
ENV_FILE:=.env
MIG:=$(shell date '+%m%d%Y.%H%M%S')
SHELL := $(shell which bash)
VERBOSE ?= 1
export REMOTE_UTILS_DIR:=./remote_utils/bin
export VALIDATE_USER_UTIL_SRC:=./internal/remote/deployment/validate
export USERS_UTIL_SRC:=./internal/remote/deployment/createuser
ifeq ($(VERBOSE),1)
	V = -v
endif
export LATEST_TAG := $(shell git fetch --tags && git tag -l "v[0-9]*.[0-9]*.[0-9]*" | sort -V | tail -n 1)

sqlc-and-migrations:
	source config_goose.sh
	goose down -v
	goose up -v
	sqlc generate


utils-dir:
	@echo "[INFO] **** Creating $(REMOTE_UTILS_DIR) ****"
	@mkdir -p $(REMOTE_UTILS_DIR)

build-validate:
	@echo "[INFO] **** building validate-user utility outdir: $(REMOTE_UTILS_DIR) src: $(VALIDATE_USER_UTIL_SRC)"
	@go build -o $(REMOTE_UTILS_DIR)/deploy-utils $(VALIDATE_USER_UTIL_SRC) && chmod +x $(REMOTE_UTILS_DIR)/deploy-utils
	@go build -o $(REMOTE_UTILS_DIR)/user-utils $(USERS_UTIL_SRC) && chmod +x $(REMOTE_UTILS_DIR)/user-utils

utils: utils-dir build-validate
	@echo "[INFO] **** Building remote utils ****"

build: utils
	@echo "[INFO] Creating $(ARTIFACT_DIR)..."
	@mkdir -p $(ARTIFACT_DIR)
	@echo "[INFO] Building new release artifact binary: $(ARTIFACT)"
	@go build $(V) -o $(ARTIFACT) .

build-quiet: utils
	go build -o $(BIN_NAME)

install: build
	@echo "[INFO] ensuring install path: $(INSTALL_PATH) exists..."
	@mkdir -p $(INSTALL_PATH)
	@echo "[INFO] creating the default config dir: $(DEFUALT_CONFIG_DIR)"
	@mkdir -p $(DEFUALT_CONFIG_DIR)
	@echo "[INFO] Copying default config file: $(DEFAULT_CFG_FILE) to $(DEFUALT_CONFIG_DIR)"
	@cp $(DEFAULT_CFG_FILE) $(DEFUALT_CONFIG_DIR)
	@echo "[INFO] moving release artifact: $(ARTIFACT) to $(INSTALL_PATH)/$(BIN_NAME)"
	@mv $(ARTIFACT) $(INSTALL_PATH)/$(BIN_NAME)

# Add this target to the end of your Makefile
.PHONY: build-validate utils-dir utils build build-quiet install fetch-tags

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


buildandpush-dbhelper: check-builder
	docker buildx use goinfaclibuilder
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DBHELPERAPI_GHCR_REPO)$(tag) . --push
