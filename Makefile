.PHONY: all all_crosscompile install build_frontend build_backend build build_crosscompile build_webdist package_webdist build_docker build_dev_docker run_dev_docker watch_frontend watch_backend watch clean clean_docker check_changes go_mod fmt test check-tr update_js_deps update_go_deps

# Specify the name of your Go binary output
BINARY_NAME := opengist
GIT_TAG := $(shell git describe --tags)
VERSION := $(patsubst v%,%,$(GIT_TAG))
VERSION_PKG := github.com/thomiceli/opengist/internal/config.OpengistVersion
TEST_DB_TYPE ?= sqlite

all: clean install build

all_crosscompile: clean install build_frontend build_crosscompile

install:
	@echo "Installing NPM dependencies..."
	@npm ci || (echo "Error: Failed to install NPM dependencies." && exit 1)
	@echo "Installing Go dependencies..."
	@go mod download || (echo "Error: Failed to install Go dependencies." && exit 1)

build_frontend:
	@echo "Building frontend assets..."
	npx vite -c public/vite.config.js build

build_backend:
	@echo "Building Opengist binary..."
	go build -tags fs_embed -ldflags "-X $(VERSION_PKG)=$(GIT_TAG)" -o $(BINARY_NAME) .

build: build_frontend build_backend

build_crosscompile:
	@bash ./scripts/build-all.sh

# Package the built, platform-independent frontend assets (the files embedded
# via //go:embed in public/fs_embed.go) into a tarball. Downstream packagers
# (e.g. distro ports) can fetch this instead of running the npm/vite toolchain.
# Use build_webdist to build assets first; package_webdist assumes they exist.
build_webdist: build_frontend package_webdist

package_webdist:
	@test -f public/.vite/manifest.json || (echo "Error: frontend assets not found in public/. Run 'make build_frontend' first." && exit 1)
	@echo "Packaging frontend assets into build/$(BINARY_NAME)$(VERSION)-webdist.tar.gz..."
	@mkdir -p build
	@tar -czf "build/$(BINARY_NAME)$(VERSION)-webdist.tar.gz" \
		--transform 's,^,$(BINARY_NAME)-webdist/,' \
		-C public .vite/manifest.json assets
	@sha256sum "build/$(BINARY_NAME)$(VERSION)-webdist.tar.gz" | awk '{print $$1 " " substr($$2,7)}' >> build/checksums.txt
	@echo "Done."

build_docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

build_dev_docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME)-dev:latest --target dev .

run_dev_docker:
	docker run -v .:/opengist -v /opengist/node_modules -p 6157:6157 -p 16157:16157 -p 2222:2222 -v $(HOME)/.opengist-dev:/root/.opengist --rm $(BINARY_NAME)-dev:latest

watch_frontend:
	@echo "Building frontend assets..."
	npx vite -c public/vite.config.js --port 16157 --host

watch_backend:
	@echo "Building Opengist binary..."
	OG_DEV=1 npx nodemon --watch '**/*' -e html,yml,go,js --signal SIGTERM --exec 'go run -ldflags "-X $(VERSION_PKG)=$(GIT_TAG)" . --config config.yml'

watch:
	@sh ./scripts/watch.sh

clean:
	@echo "Cleaning up build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf public/assets public/.vite build

clean_docker:
	@echo "Cleaning up Docker image..."
	@docker rmi $(BINARY_NAME)

check_changes:
	@echo "Checking for changes..."
	@git --no-pager diff --exit-code || (echo "There are unstaged changes detected." && exit 1)

go_mod:
	@go mod download
	@go mod tidy

fmt:
	@go fmt ./...

test:
	@OPENGIST_TEST_DB=$(TEST_DB_TYPE) go test ./... -p 1

check-tr:
	@bash ./scripts/check-translations.sh

update_js_deps:
	@echo "Updating NPM dependencies..."
	@npx npm-check-updates -u && npm install

update_go_deps:
	@echo "Updating Go dependencies..."
	@go get -u ./... && go mod tidy