GHCR_REPO:=ghcr.io/babbage88/infractl:
OS_ARCH:=$(shell uname)
ARTIFACT_DIR=dist
BIN_NAME:=infractl
ARTIFACT:=$(ARTIFACT_DIR)/$(BIN_NAME)
DEFAULT_CONFIG_DIR:=~/.config/infractl
DEFAULT_CFG_FILE:=default.yaml
ifeq ($(OS_ARCH),Darwin)
	DEFAULT_CONFIG_DIR = $(HOME)/Library/Application Support/infractl
else ifeq ($(OS_ARCH),Linux)
	DEFAULT_CONFIG_DIR = $(HOME)/.config/infractl
else
	$(error "Unsupported OS: $(OS_ARCH)")
endif
PVE_CONFIG_FILE:=pve.yaml
DEFAULT_CFG_PATH=$(DEFAULT_CONFIG_DIR)/$(DEFAULT_CFG_FILE)
PVE_CONFIG_PATH=$(DEFAULT_CONFIG_DIR)/$(PVE_CONFIG_FILE)
MAIN_BRANCH:=master
VERSION_TYPE:=patch
export CUR_USER:=$(shell whoami)
GOPATH:=$(HOME)/go
GOBIN:=$(GOPATH)/bin
INSTALL_PATH:=$(HOME)/go/bin
ENV_FILE:=.env
MIG:=$(shell date '+%m%d%Y.%H%M%S')
VERBOSE ?= 1
export REMOTE_UTILS_DIR:=./remote_utils/bin
export VALIDATE_USER_UTIL_SRC:=./internal/remote/deployment/validate
export USERS_UTIL_SRC:=./internal/remote/deployment/createuser
REMOTE_UTILS_PLATFORMS ?= linux/amd64 linux/arm64
BUILD_GOOS ?= $(shell go env GOOS)
BUILD_GOARCH ?= $(shell go env GOARCH)
RELEASE_ARTIFACT_NAME = $(BIN_NAME)_$(BUILD_GOOS)_$(BUILD_GOARCH)
RELEASE_STAGE_DIR = $(ARTIFACT_DIR)/$(RELEASE_ARTIFACT_NAME)
RELEASE_ARCHIVE = $(ARTIFACT_DIR)/$(RELEASE_ARTIFACT_NAME).tar.gz
ifeq ($(VERBOSE),1)
	V = -v
endif
release_build_flags:=-trimpath -ldflags="-s -w"

sqlc-and-migrations:
	source config_goose.sh
	goose down -v
	goose up -v
	sqlc generate

utils-dir:
	@echo "[INFO] **** Creating $(REMOTE_UTILS_DIR) ****"
	@mkdir -p $(REMOTE_UTILS_DIR)

build-validate: utils-dir
	@echo "[INFO] **** building remote utils for platforms: $(REMOTE_UTILS_PLATFORMS) ****"
	@for platform in $(REMOTE_UTILS_PLATFORMS); do \
		goos=$${platform%/*}; \
		goarch=$${platform#*/}; \
		outdir="$(REMOTE_UTILS_DIR)/$${goos}-$${goarch}"; \
		echo "[INFO] **** building deploy-utils for $$goos/$$goarch -> $$outdir ****"; \
		mkdir -p "$$outdir"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -o "$$outdir/deploy-utils" $(VALIDATE_USER_UTIL_SRC); \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build -o "$$outdir/user-utils" $(USERS_UTIL_SRC); \
		chmod +x "$$outdir/deploy-utils" "$$outdir/user-utils"; \
	done

utils: build-validate
	@echo "[INFO] **** Built embedded remote utils ****"

build: utils
	@echo "[INFO] Creating $(ARTIFACT_DIR)..."
	@mkdir -p $(ARTIFACT_DIR)
	@echo "[INFO] Building local artifact binary: $(ARTIFACT)"
	@CGO_ENABLED=0 GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build $(V) -o $(ARTIFACT) .

build-release: utils
	@echo "[INFO] Creating $(ARTIFACT_DIR)..."
	@mkdir -p $(ARTIFACT_DIR)
	@echo "[INFO] Building release artifact binary: $(ARTIFACT)"
	@CGO_ENABLED=0 GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build $(release_build_flags) $(V) -o $(ARTIFACT) .

release-artifact: utils
	@echo "[INFO] Creating release archive for $(BUILD_GOOS)/$(BUILD_GOARCH)..."
	@mkdir -p $(RELEASE_STAGE_DIR)
	@CGO_ENABLED=0 GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build $(release_build_flags) $(V) -o $(RELEASE_STAGE_DIR)/$(BIN_NAME) .
	@tar -C $(ARTIFACT_DIR) -czf $(RELEASE_ARCHIVE) $(RELEASE_ARTIFACT_NAME)
	@{ \
		if command -v sha256sum >/dev/null 2>&1; then \
			sha256sum "$(RELEASE_ARCHIVE)" > "$(RELEASE_ARCHIVE).sha256"; \
		else \
			shasum -a 256 "$(RELEASE_ARCHIVE)" > "$(RELEASE_ARCHIVE).sha256"; \
		fi; \
	}

build-quiet: utils
	@CGO_ENABLED=0 GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build -o $(BIN_NAME)

install: build
	@echo "[INFO] ensuring install path: $(INSTALL_PATH) exists..."
	@mkdir -p $(INSTALL_PATH)
	@echo "[INFO] creating the default config dir: $(DEFAULT_CONFIG_DIR)"
	@mkdir -p "$(DEFAULT_CONFIG_DIR)"
	@echo "[INFO] Checking if $(DEFAULT_CFG_PATH) exists..."
	@if [ ! -f "$(DEFAULT_CFG_PATH)" ]; then \
		echo "[INFO] Copying default config file: $(DEFAULT_CFG_FILE) to $(DEFAULT_CONFIG_DIR)"; \
		cp "$(DEFAULT_CFG_FILE)" "$(DEFAULT_CONFIG_DIR)"; \
	fi
	@echo "[INFO] Checking if $(PVE_CONFIG_PATH) exists..."
	@if [ ! -f "$(PVE_CONFIG_PATH)" ]; then \
		echo "[INFO] Copying default pve config file: $(PVE_CONFIG_FILE) to $(DEFAULT_CONFIG_DIR)"; \
		cp "$(PVE_CONFIG_FILE)" "$(DEFAULT_CONFIG_DIR)"; \
	fi
	@echo "[INFO] moving release artifact: $(ARTIFACT) to $(INSTALL_PATH)/$(BIN_NAME)"
	@mv "$(ARTIFACT)" "$(INSTALL_PATH)/$(BIN_NAME)"

.PHONY: build-validate utils-dir utils build build-quiet build-release release-artifact install fetch-tags check-builder create-builder buildandpush buildandpush-dbhelper

fetch-tags:
	@{ \
	MAIN_BRANCH=$(shell echo "$(VERSION)");branch=$$(git rev-parse --abbrev-ref HEAD); \
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
		latest_tag=$$(git tag -l "v[0-9]*.[0-9]*.[0-9]*" | sort -V | tail -n 1); \
		echo "Latest tag: $$latest_tag"; \
		new_tag=$$(go run . utils version-bumper --latest-version "$$latest_tag" --increment-type=$(VERSION_TYPE)); \
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
